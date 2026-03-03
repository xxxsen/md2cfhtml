package md2cfhtml

import (
	"os"
	"strings"
	"testing"
)

func TestConvertTOCMermaidAndCode(t *testing.T) {
	input := `# Demo

[TOC]

` + "```mermaid\nflowchart TD\nA-->B\n```\n\n```go\nfmt.Println(\"hi\")\n```\n"

	output, err := ConvertString(input)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if !strings.Contains(output, `<ac:structured-macro ac:name="toc"></ac:structured-macro>`) {
		t.Fatalf("toc macro not found in output")
	}
	if !strings.Contains(output, `<ac:structured-macro ac:name="mermaid-macro">`) {
		t.Fatalf("mermaid macro not found in output")
	}
	if !strings.Contains(output, `<ac:structured-macro ac:name="code">`) {
		t.Fatalf("code macro not found in output")
	}
	if !strings.Contains(output, `<ac:parameter ac:name="language">go</ac:parameter>`) {
		t.Fatalf("go language parameter not found in output")
	}
}

func TestConvertSampleMarkdown(t *testing.T) {
	data, err := os.ReadFile("sample/test.md")
	if err != nil {
		t.Fatalf("read sample failed: %v", err)
	}

	output, err := Convert(data)
	if err != nil {
		t.Fatalf("convert sample failed: %v", err)
	}

	html := string(output)
	if !strings.Contains(html, `<ac:structured-macro ac:name="toc"></ac:structured-macro>`) {
		t.Fatalf("toc macro not found in converted sample")
	}
	if strings.Count(html, `<ac:structured-macro ac:name="mermaid-macro">`) < 4 {
		t.Fatalf("expected at least 4 mermaid blocks, got fewer")
	}
	if !strings.Contains(html, `<ac:structured-macro ac:name="code">`) {
		t.Fatalf("code macro not found in converted sample")
	}
}
