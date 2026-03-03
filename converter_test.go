package md2cfhtml

import (
	"os"
	"strings"
	"testing"
)

func TestConvertTOCMermaidPlantUMLAndCode(t *testing.T) {
	input := `# Demo

[TOC]

` + "```mermaid\nflowchart TD\nA-->B\n```\n\n```plantuml\n@startuml\nAlice -> Bob: hello\n@enduml\n```\n\n```go\nfmt.Println(\"hi\")\n```\n"

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
	if !strings.Contains(output, `<ac:structured-macro ac:name="plantuml">`) {
		t.Fatalf("plantuml macro not found in output")
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

func TestConvertPlantUMLAliasAndMacroNameOverride(t *testing.T) {
	input := "```puml\n@startuml\nA -> B\n@enduml\n```"
	output, err := ConvertString(input, WithPlantUMLMacroName("plantumlrender"))
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if !strings.Contains(output, `<ac:structured-macro ac:name="plantumlrender">`) {
		t.Fatalf("custom plantuml macro not found in output")
	}
}

func TestConvertTableHeaderIncludesHeaderRow(t *testing.T) {
	input := "| Name | Age |\n|---|---|\n| Alice | 18 |\n"
	output, err := ConvertString(input)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if !strings.Contains(output, "<thead>\n<tr>\n") {
		t.Fatalf("table header row not found")
	}
	if !strings.Contains(output, "</tr>\n</thead>\n") {
		t.Fatalf("table header row closing tags not found")
	}
	if !strings.Contains(output, "<th>Name</th>") || !strings.Contains(output, "<th>Age</th>") {
		t.Fatalf("table header cells not rendered as <th>")
	}
	if !strings.Contains(output, "<td>Alice</td>") || !strings.Contains(output, "<td>18</td>") {
		t.Fatalf("table body cells not rendered as <td>")
	}
}

func TestConvertAutoLinkEmailUsesMailto(t *testing.T) {
	output, err := ConvertString("<foo@example.com>\n")
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if !strings.Contains(output, `<a href="mailto:foo@example.com">foo@example.com</a>`) {
		t.Fatalf("email autolink is missing mailto prefix")
	}
}

func TestConvertTaskListToConfluenceTasks(t *testing.T) {
	input := "- [x] done\n- [ ] todo\n"
	output, err := ConvertString(input)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if !strings.Contains(output, "<ac:task-list>") {
		t.Fatalf("task list wrapper not rendered")
	}
	if !strings.Contains(output, "<ac:task-status>complete</ac:task-status>") {
		t.Fatalf("checked task status not rendered")
	}
	if !strings.Contains(output, "<ac:task-status>incomplete</ac:task-status>") {
		t.Fatalf("unchecked task status not rendered")
	}
	if !strings.Contains(output, "<ac:task-body>done</ac:task-body>") {
		t.Fatalf("checked task body not rendered")
	}
	if !strings.Contains(output, "<ac:task-body>todo</ac:task-body>") {
		t.Fatalf("unchecked task body not rendered")
	}
	if strings.Contains(output, "[x]") || strings.Contains(output, "[ ]") {
		t.Fatalf("task marker text should not appear in task-list mode")
	}
}

func TestConvertNormalListUnaffected(t *testing.T) {
	output, err := ConvertString("- item1\n- item2\n")
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if !strings.Contains(output, "<ul>") || !strings.Contains(output, "<li>item1</li>") {
		t.Fatalf("normal bullet list should still use ul/li")
	}
}

func TestConvertAdmonitionToConfluenceMacro(t *testing.T) {
	input := ":::warning\n\nhello **world**\n\n:::\n"
	output, err := ConvertString(input)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if !strings.Contains(output, `<ac:structured-macro ac:name="warning">`) {
		t.Fatalf("warning macro not rendered")
	}
	if !strings.Contains(output, "<ac:rich-text-body>") {
		t.Fatalf("warning rich-text-body not rendered")
	}
	if !strings.Contains(output, "<p>hello <strong>world</strong></p>") {
		t.Fatalf("warning body not rendered correctly")
	}
	if strings.Contains(output, ":::warning") || strings.Contains(output, "<p>:::</p>") {
		t.Fatalf("admonition markers should not appear in output")
	}
}

func TestConvertTableAlignment(t *testing.T) {
	input := "| L | C | R |\n|:--|:-:|--:|\n|1|2|3|\n"
	output, err := ConvertString(input)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if !strings.Contains(output, `<th style="text-align:left;">L</th>`) {
		t.Fatalf("left alignment not rendered on header")
	}
	if !strings.Contains(output, `<th style="text-align:center;">C</th>`) {
		t.Fatalf("center alignment not rendered on header")
	}
	if !strings.Contains(output, `<th style="text-align:right;">R</th>`) {
		t.Fatalf("right alignment not rendered on header")
	}
	if !strings.Contains(output, `<td style="text-align:left;">1</td>`) {
		t.Fatalf("left alignment not rendered on body cell")
	}
	if !strings.Contains(output, `<td style="text-align:center;">2</td>`) {
		t.Fatalf("center alignment not rendered on body cell")
	}
	if !strings.Contains(output, `<td style="text-align:right;">3</td>`) {
		t.Fatalf("right alignment not rendered on body cell")
	}
}
