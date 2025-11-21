package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
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
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "无法读取上传文件，请重试", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "请选择要转换的文件", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempDir, err := os.MkdirTemp("", "whx-upload-")
	if err != nil {
		http.Error(w, "无法创建临时目录", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, filepath.Base(header.Filename))
	inputFile, err := os.Create(inputPath)
	if err != nil {
		http.Error(w, "无法保存上传文件", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(inputFile, file); err != nil {
		inputFile.Close()
		http.Error(w, "无法保存上传文件", http.StatusInternalServerError)
		return
	}
	if err := inputFile.Close(); err != nil {
		http.Error(w, "无法保存上传文件", http.StatusInternalServerError)
		return
	}

	outputPath, _, err := Generate(inputPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("转换失败: %v", err), http.StatusBadRequest)
		return
	}

	result, err := os.Open(outputPath)
	if err != nil {
		http.Error(w, "无法读取转换结果", http.StatusInternalServerError)
		return
	}
	defer result.Close()

	filename := filepath.Base(outputPath)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if _, err := io.Copy(w, result); err != nil {
		return
	}
}

func init() {
	if _, err := webFS.ReadFile("web/index.html"); err != nil {
		panic(fmt.Errorf("embedded index page missing: %w", err))
	}
}
