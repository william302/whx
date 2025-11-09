package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	logisticsBrand = "First Logistics"
	orderPlatform  = "SHOPIFY"
	outboundType   = "销售出库"
	defaultMapPath = "map.xlsx"
	mapPathEnvVar  = "MAP_PATH"
	resultFileName = "result.xlsx"
)

var (
	packageDir string
	repoRoot   string
)

func init() {
	if _, file, _, ok := runtime.Caller(0); ok {
		packageDir = filepath.Dir(file)
		repoRoot = filepath.Clean(filepath.Join(packageDir, "..", ".."))
	} else {
		packageDir = "."
		repoRoot = "."
	}
}

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
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <path/to/input.xlsx>", filepath.Base(os.Args[0]))
	}
	outputPath, count, err := Generate(os.Args[1])
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

	mapPath, err := resolveMapPath(inputPath)
	if err != nil {
		return "", 0, err
	}

	skuMap, err := loadMapping(mapPath)
	if err != nil {
		return "", 0, fmt.Errorf("load sku map: %w", err)
	}

	rows, err := buildRows(inputPath, skuMap)
	if err != nil {
		return "", 0, fmt.Errorf("prepare rows: %w", err)
	}

	outputPath := filepath.Join(filepath.Dir(inputPath), resultFileName)
	if err := writeOutput(rows, outputPath); err != nil {
		return "", 0, fmt.Errorf("write workbook: %w", err)
	}

	abs, err := filepath.Abs(outputPath)
	if err == nil {
		outputPath = abs
	}
	return outputPath, len(rows), nil
}

func loadMapping(path string) (map[string]string, error) {
	f, err := excelize.OpenFile(path)
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

	var result []outputRow
	for i, row := range rows {
		if i == 0 {
			continue // skip header
		}
		get := func(idx int) string {
			if idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}
		orderID := get(0)
		sku := get(1)
		qtyStr := get(2)
		method := get(3)
		tracking := get(4)
		country := get(5)

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
			"",
		}
		if row.HasTracking {
			values[0] = outboundType
			values[2] = logisticsBrand
			values[3] = row.LogisticsChannel
			values[6] = orderPlatform
			values[7] = row.CustomerRef
			values[12] = row.Country
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

func resolveMapPath(inputPath string) (string, error) {
	candidates := []string{}
	if envPath := os.Getenv(mapPathEnvVar); envPath != "" {
		candidates = append(candidates, envPath)
	}
	candidates = append(candidates, defaultMapPath)

	inputDir := filepath.Dir(inputPath)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		path := candidate
		if !filepath.IsAbs(path) {
			path = filepath.Clean(path)
		}
		if infoPath, err := existingPath(path); err == nil {
			return infoPath, nil
		}
		// try relative to input directory
		if !filepath.IsAbs(candidate) {
			alt := filepath.Join(inputDir, filepath.Base(candidate))
			if infoPath, err := existingPath(alt); err == nil {
				return infoPath, nil
			}
			if repoRoot != "" {
				rootPath := filepath.Join(repoRoot, candidate)
				if infoPath, err := existingPath(rootPath); err == nil {
					return infoPath, nil
				}
			}
		}
	}
	return "", fmt.Errorf("map.xlsx not found (checked %s)", strings.Join(candidates, ", "))
}

func existingPath(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}
