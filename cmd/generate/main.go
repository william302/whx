package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	inputPath      = "表格格式/1103/input.xlsx"
	mapPath        = "map.xlsx"
	outputPath     = "result.excel"
	logisticsBrand = "First Logistics"
	orderPlatform  = "SHOPIFY"
	outboundType   = "销售出库"
)

type outputRow struct {
	Tracking         string
	LogisticsChannel string
	SKUCode          string
	Quantity         int
	CustomerRef      string
	Country          string
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	skuMap, err := loadMapping(mapPath)
	if err != nil {
		return fmt.Errorf("load sku map: %w", err)
	}

	rows, err := buildRows(inputPath, skuMap)
	if err != nil {
		return fmt.Errorf("prepare rows: %w", err)
	}

	if err := writeOutput(rows, outputPath); err != nil {
		return fmt.Errorf("write workbook: %w", err)
	}

	abs, err := filepath.Abs(outputPath)
	if err != nil {
		return nil
	}
	fmt.Printf("Created %s with %d rows\n", abs, len(rows))
	return nil
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

		if sku == "" && tracking == "" {
			continue
		}
		if orderID == "" {
			return nil, fmt.Errorf("row %d: missing platform order number in column A", i+1)
		}
		if sku == "" {
			return nil, fmt.Errorf("row %d: missing SKU in column B", i+1)
		}
		if tracking == "" {
			return nil, fmt.Errorf("row %d: missing tracking number in column E", i+1)
		}
		code, ok := skuMap[sku]
		if !ok {
			return nil, fmt.Errorf("row %d: sku %q not found in %s", i+1, sku, mapPath)
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
			outboundType,
			row.Tracking,
			logisticsBrand,
			row.LogisticsChannel,
			row.SKUCode,
			row.Quantity,
			orderPlatform,
			row.CustomerRef,
			"",
			"",
			"",
			"",
			row.Country,
		}
		cell := fmt.Sprintf("A%d", i+2)
		if err := f.SetSheetRow(sheet, cell, &values); err != nil {
			return fmt.Errorf("set row %d: %w", i+2, err)
		}
	}

	savePath := path
	renameTo := ""
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		savePath = path + ".tmp.xlsx"
		renameTo = path
	}

	if err := f.SaveAs(savePath); err != nil {
		return err
	}
	if renameTo != "" {
		if err := os.Remove(renameTo); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale %s: %w", renameTo, err)
		}
		if err := os.Rename(savePath, renameTo); err != nil {
			return fmt.Errorf("rename %s to %s: %w", savePath, renameTo, err)
		}
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
