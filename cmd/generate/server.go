package main

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/xuri/excelize/v2"
)

const maxUploadSize = 25 << 20 // 25 MB guardrail for uploads

//go:embed web/index.html
var webFS embed.FS

type changelogEntry struct {
	Version string
	Date    string
	Items   []string
}

var changelog = []changelogEntry{
	{
		Version: "0.4.0",
		Date:    "2024-11",
		Items: []string{
			"更新 SKU 映射至 2024-11-27 版",
			"命令行与网页版本号同步到 0.4.0",
		},
	},
	{
		Version: "0.3.0",
		Date:    "2024-06",
		Items: []string{
			"预览弹窗展示全量数据，预览与下载分离",
			"上传提示与下载按钮样式强化，指引更明显",
			"默认端口改为 8001，版本同步到 0.3.0",
		},
	},
	{
		Version: "0.2.0",
		Date:    "2024-05",
		Items: []string{
			"新增网页端上传与下载，直接转换出仓库文件",
			"现在根据列名从输入的Excel查找数据",
			"省缺的记录添加国家字段",
			"页面展示当前版本与更新记录，方便查看",
		},
	},
	{
		Version: "0.1.0",
		Date:    "2024-04",
		Items: []string{
			"命令行工具初始发布，支持 SKU 映射转换",
		},
	},
}

func serveWeb(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/convert", handleConvert)
	mux.HandleFunc("/api/preview", handlePreview)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	fmt.Printf("Serving WHX %s on %s\n", versionString, addr)
	return srv.ListenAndServe()
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	markup, err := webFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "template missing", http.StatusInternalServerError)
		return
	}
	tmpl, err := template.New("index").Parse(string(markup))
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data := struct {
		Version   string
		Changelog []changelogEntry
	}{
		Version:   versionString,
		Changelog: changelog,
	}
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

func handleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conv, err := prepareConversion(w, r)
	if err != nil {
		return
	}
	defer conv.cleanup()

	result, err := os.Open(conv.outputPath)
	if err != nil {
		http.Error(w, "无法读取转换结果", http.StatusInternalServerError)
		return
	}
	defer result.Close()

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", conv.filename))
	if _, err := io.Copy(w, result); err != nil {
		return
	}
}

func handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conv, err := prepareConversion(w, r)
	if err != nil {
		return
	}
	defer conv.cleanup()

	preview, err := buildPreview(conv.outputPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("无法生成预览: %v", err), http.StatusInternalServerError)
		return
	}

	fileBytes, err := os.ReadFile(conv.outputPath)
	if err != nil {
		http.Error(w, "无法读取转换结果", http.StatusInternalServerError)
		return
	}

	payload := struct {
		Filename string        `json:"filename"`
		File     string        `json:"file"`
		Preview  previewResult `json:"preview"`
	}{
		Filename: conv.filename,
		File:     base64.StdEncoding.EncodeToString(fileBytes),
		Preview:  preview,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return
	}
}

func init() {
	if _, err := webFS.ReadFile("web/index.html"); err != nil {
		panic(fmt.Errorf("embedded index page missing: %w", err))
	}
}

type previewResult struct {
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

type conversionResult struct {
	outputPath string
	filename   string
	cleanup    func()
}

func prepareConversion(w http.ResponseWriter, r *http.Request) (conversionResult, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "无法读取上传文件，请重试", http.StatusBadRequest)
		return conversionResult{}, err
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "请选择要转换的文件", http.StatusBadRequest)
		return conversionResult{}, err
	}
	defer file.Close()

	tempDir, err := os.MkdirTemp("", "whx-upload-")
	if err != nil {
		http.Error(w, "无法创建临时目录", http.StatusInternalServerError)
		return conversionResult{}, err
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	inputPath := filepath.Join(tempDir, filepath.Base(header.Filename))
	inputFile, err := os.Create(inputPath)
	if err != nil {
		http.Error(w, "无法保存上传文件", http.StatusInternalServerError)
		cleanup()
		return conversionResult{}, err
	}
	if _, err := io.Copy(inputFile, file); err != nil {
		inputFile.Close()
		http.Error(w, "无法保存上传文件", http.StatusInternalServerError)
		cleanup()
		return conversionResult{}, err
	}
	if err := inputFile.Close(); err != nil {
		http.Error(w, "无法保存上传文件", http.StatusInternalServerError)
		cleanup()
		return conversionResult{}, err
	}

	outputPath, _, err := Generate(inputPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("转换失败: %v", err), http.StatusBadRequest)
		cleanup()
		return conversionResult{}, err
	}

	return conversionResult{
		outputPath: outputPath,
		filename:   filepath.Base(outputPath),
		cleanup:    cleanup,
	}, nil
}

func buildPreview(path string) (previewResult, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return previewResult{}, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return previewResult{}, fmt.Errorf("output missing sheets")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return previewResult{}, err
	}
	if len(rows) == 0 {
		return previewResult{}, fmt.Errorf("output empty")
	}

	headers := rows[0]
	data := make([][]string, 0, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		record := make([]string, len(headers))
		for j := range headers {
			if j < len(row) {
				record[j] = row[j]
			} else {
				record[j] = ""
			}
		}
		data = append(data, record)
	}

	return previewResult{Headers: headers, Rows: data}, nil
}
