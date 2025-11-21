package main

import (
	"bytes"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	logisticsBrand = "First Logistics"
	orderPlatform  = "SHOPIFY"
	outboundType   = "销售出库"
	outputPrefix   = "Warehouse_"
	versionString  = "0.2.0"
)

//go:embed map.xlsx
var embeddedMap []byte

type outputRow struct {
	Tracking         string
	LogisticsChannel string
	SKUCode          string
	Quantity         int
	CustomerRef      string
	Country          string
	HasTracking      bool
}

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	serve := flag.Bool("serve", false, "start web server for uploads")
	addr := flag.String("addr", ":8080", "listen address in serve mode")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [--version] [--serve] [--addr :8080] <path/to/input.xlsx>\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(versionString)
		return
	}

	if *serve {
		if flag.NArg() != 0 {
			flag.Usage()
			os.Exit(1)
		}
		if err := serveWeb(*addr); err != nil {
			log.Fatal(err)
		}
		return
	}

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	outputPath, count, err := Generate(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created %s with %d rows\n", outputPath, count)
}

// Generate builds the result.xlsx file next to the provided input workbook.
func Generate(inputPath string) (string, int, error) {
	if inputPath == "" {
		return "", 0, errors.New("input path is required")
	}

	skuMap, err := loadMapping()
	if err != nil {
		return "", 0, fmt.Errorf("load sku map: %w", err)
	}

	rows, err := buildRows(inputPath, skuMap)
	if err != nil {
		return "", 0, fmt.Errorf("prepare rows: %w", err)
	}

	outputName := outputPrefix + filepath.Base(inputPath)
	outputPath := filepath.Join(filepath.Dir(inputPath), outputName)
	if err := writeOutput(rows, outputPath); err != nil {
		return "", 0, fmt.Errorf("write workbook: %w", err)
	}

	abs, err := filepath.Abs(outputPath)
	if err == nil {
		outputPath = abs
	}
	return outputPath, len(rows), nil
}

func loadMapping() (map[string]string, error) {
	if len(embeddedMap) == 0 {
		return nil, errors.New("embedded map.xlsx is empty")
	}
	f, err := excelize.OpenReader(bytes.NewReader(embeddedMap))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("mapping workbook has no sheets")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}

	mapping := make(map[string]string, len(rows))
	for i, row := range rows {
		if i == 0 || len(row) < 2 {
			continue
		}
		sku := strings.TrimSpace(row[0])
		code := strings.TrimSpace(row[1])
		if sku == "" || code == "" {
			continue
		}
		mapping[sku] = code
	}
	if len(mapping) == 0 {
		return nil, errors.New("mapping workbook is empty")
	}
	return mapping, nil
}

func buildRows(path string, skuMap map[string]string) ([]outputRow, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("input workbook has no sheets")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}

	cols, err := detectColumns(rows)
	if err != nil {
		return nil, err
	}

	var result []outputRow
	lastCountry := make(map[string]string)
	for i, row := range rows {
		if i == 0 {
			continue // skip header
		}
		get := func(idx int) string {
			if idx < 0 || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}
		orderID := get(cols.order)
		sku := get(cols.sku)
		qtyStr := get(cols.qty)
		method := get(cols.method)
		tracking := get(cols.tracking)
		country := get(cols.country)
		if country == "" && orderID != "" {
			country = lastCountry[orderID]
		}

		if sku == "" {
			continue
		}
		if qtyStr == "" {
			continue
		}
		code, ok := skuMap[sku]
		if !ok {
			return nil, fmt.Errorf("row %d: sku %q not found in mapping", i+1, sku)
		}
		qty, err := strconv.Atoi(qtyStr)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid quantity %q: %w", i+1, qtyStr, err)
		}
		if country != "" && orderID != "" {
			lastCountry[orderID] = country
		}
		result = append(result, outputRow{
			Tracking:         tracking,
			LogisticsChannel: extractChannel(method),
			SKUCode:          code,
			Quantity:         qty,
			CustomerRef:      orderID,
			Country:          country,
			HasTracking:      tracking != "",
		})
	}
	return result, nil
}

type columnIndexes struct {
	order    int
	sku      int
	qty      int
	method   int
	tracking int
	country  int
}

func detectColumns(rows [][]string) (columnIndexes, error) {
	if len(rows) == 0 {
		return columnIndexes{}, errors.New("input workbook has no rows")
	}
	header := rows[0]
	find := func(name string) int {
		for i, cell := range header {
			if strings.TrimSpace(cell) == name {
				return i
			}
		}
		return -1
	}
	cols := columnIndexes{
		order:    find("平台单号"),
		sku:      find("SKU"),
		qty:      find("数量"),
		method:   find("物流方式"),
		tracking: find("运单号"),
		country:  find("国家/地区"),
	}
	if cols.order < 0 || cols.sku < 0 || cols.qty < 0 {
		return columnIndexes{}, errors.New("input workbook header missing required columns")
	}
	return cols, nil
}

func writeOutput(rows []outputRow, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Sheet1"
	headers := []string{"出库类型", "运单号", "物流公司", "物流渠道", "SKU编码", "数量", "订单平台", "客户参考单号", "其他参考单号", "出库优先级", "备注", "面单URL", "渠道国家"}
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		return err
	}

	for i, row := range rows {
		values := []interface{}{
			"", // 出库类型
			row.Tracking,
			"", // 物流公司
			"", // 物流渠道
			row.SKUCode,
			row.Quantity,
			"", // 订单平台
			"", // 客户参考单号
			"",
			"",
			"",
			"",
			row.Country,
		}
		if row.HasTracking {
			values[0] = outboundType
			values[2] = logisticsBrand
			values[3] = row.LogisticsChannel
			values[6] = orderPlatform
			values[7] = row.CustomerRef
		}

		cell := fmt.Sprintf("A%d", i+2)
		if err := f.SetSheetRow(sheet, cell, &values); err != nil {
			return fmt.Errorf("set row %d: %w", i+2, err)
		}
	}

	if err := f.SaveAs(path); err != nil {
		return err
	}
	return nil
}

func extractChannel(method string) string {
	method = strings.TrimSpace(method)
	if method == "" {
		return ""
	}
	parts := strings.SplitN(method, "-", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return method
}
