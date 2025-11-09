package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestGenerateFixtures(t *testing.T) {
	root := testRepoRoot(t)
	testCases := []struct {
		name   string
		subdir string
	}{
		{
			name:   "1008",
			subdir: filepath.Join("表格格式", "1008"),
		},
		{
			name:   "1103",
			subdir: filepath.Join("表格格式", "1103"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			inputPath := filepath.Join(root, tc.subdir, "input.xlsx")
			expectedPath := filepath.Join(root, tc.subdir, "output.xlsx")
			resultPath := filepath.Join(filepath.Dir(inputPath), "result.xlsx")
			t.Cleanup(func() {
				_ = os.Remove(resultPath)
			})

			gotPath, _, err := Generate(inputPath)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			expectedAbs, err := filepath.Abs(resultPath)
			if err != nil {
				t.Fatalf("abs result path: %v", err)
			}
			if gotPath != expectedAbs {
				t.Fatalf("expected result at %s, got %s", expectedAbs, gotPath)
			}

			compareWorkbooks(t, expectedPath, gotPath)
		})
	}
}

func compareWorkbooks(t *testing.T, expectedPath, actualPath string) {
	t.Helper()
	const sheetIndex = 0
	cols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "M"}

	want, err := extractColumns(expectedPath, sheetIndex, cols)
	if err != nil {
		t.Fatalf("read expected workbook: %v", err)
	}
	got, err := extractColumns(actualPath, sheetIndex, cols)
	if err != nil {
		t.Fatalf("read actual workbook: %v", err)
	}

	if len(want) != len(got) {
		t.Fatalf("row count mismatch: expected %d rows got %d", len(want), len(got))
	}

	for i := range want {
		if !reflect.DeepEqual(want[i], got[i]) {
			t.Fatalf("row %d mismatch:\nexpected %v\ngot      %v", i+2, want[i], got[i])
		}
	}
}

func extractColumns(path string, sheetIdx int, cols []string) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if sheetIdx >= len(sheets) {
		return nil, fmt.Errorf("sheet index %d out of range", sheetIdx)
	}
	colIdx := make([]int, len(cols))
	for i, col := range cols {
		idx, err := excelize.ColumnNameToNumber(col)
		if err != nil {
			return nil, err
		}
		colIdx[i] = idx - 1
	}

	rows, err := f.GetRows(sheets[sheetIdx])
	if err != nil {
		return nil, err
	}

	var data [][]string
	for i, row := range rows {
		if i == 0 {
			continue // skip header
		}
		values := make([]string, len(cols))
		for j, idx := range colIdx {
			if idx < len(row) {
				values[j] = strings.TrimSpace(row[idx])
			} else {
				values[j] = ""
			}
		}
		data = append(data, values)
	}
	return data, nil
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine caller info")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
