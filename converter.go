package md2cfhtml

import (
	"fmt"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	fenceStartPattern    = regexp.MustCompile("^(`{3,})(.*)$")
	atxHeadingPattern    = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	orderedListPattern   = regexp.MustCompile(`^\s*(\d+)\.\s+(.+)$`)
	unorderedListPattern = regexp.MustCompile(`^\s*[-*+]\s+(.+)$`)
	tableSepPattern      = regexp.MustCompile(`^\s*\|?\s*[:\-]+(?:\s*\|\s*[:\-]+)*\s*\|?\s*$`)
	hrPattern            = regexp.MustCompile(`^\s*(\*{3,}|-{3,}|_{3,})\s*$`)
	urlPattern           = regexp.MustCompile(`^https?://[^\s<>"']+`)
	fenceLanguagePattern = regexp.MustCompile(`^[A-Za-z0-9_+\-.#]+$`)
)

// Options controls how markdown is converted to Confluence HTML.
type Options struct {
	EnableTOCMacro   bool
	TOCMacroName     string
	CodeMacroName    string
	MermaidMacroName string
}

// Option applies configuration to the converter.
type Option func(*Options)

// WithTOCMacroName overrides the TOC macro name. Default is "toc".
func WithTOCMacroName(name string) Option {
	return func(o *Options) {
		if strings.TrimSpace(name) != "" {
			o.TOCMacroName = strings.TrimSpace(name)
		}
	}
}

// WithCodeMacroName overrides the code macro name. Default is "code".
func WithCodeMacroName(name string) Option {
	return func(o *Options) {
		if strings.TrimSpace(name) != "" {
			o.CodeMacroName = strings.TrimSpace(name)
		}
	}
}

// WithMermaidMacroName overrides the mermaid macro name. Default is "mermaid".
func WithMermaidMacroName(name string) Option {
	return func(o *Options) {
		if strings.TrimSpace(name) != "" {
			o.MermaidMacroName = strings.TrimSpace(name)
		}
	}
}

// WithTOCMacroEnabled enables or disables [TOC] conversion.
func WithTOCMacroEnabled(enabled bool) Option {
	return func(o *Options) {
		o.EnableTOCMacro = enabled
	}
}

// Converter converts markdown into Confluence HTML.
type Converter struct {
	opts Options
}

func defaultOptions() Options {
	return Options{
		EnableTOCMacro:   true,
		TOCMacroName:     "toc",
		CodeMacroName:    "code",
		MermaidMacroName: "mermaid-macro",
	}
}

// NewConverter creates a converter with optional custom settings.
func NewConverter(options ...Option) *Converter {
	opts := defaultOptions()
	for _, apply := range options {
		apply(&opts)
	}
	return &Converter{opts: opts}
}

// Convert converts markdown bytes to Confluence HTML bytes.
func Convert(markdown []byte, options ...Option) ([]byte, error) {
	return NewConverter(options...).Convert(markdown)
}

// ConvertString converts a markdown string to Confluence HTML string.
func ConvertString(markdown string, options ...Option) (string, error) {
	output, err := Convert([]byte(markdown), options...)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// ConvertFile reads markdown from inputPath and writes the converted HTML to outputPath.
func ConvertFile(inputPath, outputPath string, options ...Option) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input markdown: %w", err)
	}

	output, err := Convert(data, options...)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, output, 0o644); err != nil {
		return fmt.Errorf("write output html: %w", err)
	}
	return nil
}

// Convert converts markdown bytes to Confluence HTML bytes.
func (c *Converter) Convert(markdown []byte) ([]byte, error) {
	renderer := &renderer{
		lines: splitLines(string(markdown)),
		opts:  c.opts,
	}
	htmlOutput, err := renderer.render()
	if err != nil {
		return nil, err
	}
	return []byte(htmlOutput), nil
}

type renderer struct {
	lines []string
	index int
	opts  Options
}

func (r *renderer) render() (string, error) {
	var output strings.Builder

	for r.index < len(r.lines) {
		line := r.lines[r.index]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			r.index++
			continue
		}

		if level, text := parseATXHeading(line); level > 0 {
			fmt.Fprintf(&output, "<h%d>%s</h%d>\n", level, renderInline(text), level)
			r.index++
			continue
		}

		if level, text, ok := r.parseSetextHeading(); ok {
			fmt.Fprintf(&output, "<h%d>%s</h%d>\n", level, renderInline(text), level)
			r.index += 2
			continue
		}

		if r.opts.EnableTOCMacro && strings.EqualFold(trimmed, "[TOC]") {
			fmt.Fprintf(&output, "<ac:structured-macro ac:name=\"%s\"></ac:structured-macro>\n", html.EscapeString(r.opts.TOCMacroName))
			r.index++
			continue
		}

		if fence, info, ok := parseFenceStart(line); ok {
			block, nextIndex := r.consumeFencedBlock(fence)
			r.index = nextIndex
			r.writeFencedBlock(&output, info, block)
			continue
		}

		if isTableHeader(r.lines, r.index) {
			r.writeTable(&output)
			continue
		}

		if matchesOrderedList(line) || matchesUnorderedList(line) {
			r.writeList(&output)
			continue
		}

		if strings.HasPrefix(strings.TrimLeft(line, " \t"), ">") {
			r.writeBlockquote(&output)
			continue
		}

		if hrPattern.MatchString(trimmed) {
			output.WriteString("<hr />\n")
			r.index++
			continue
		}

		r.writeParagraph(&output)
	}

	return output.String(), nil
}

func (r *renderer) parseSetextHeading() (int, string, bool) {
	if r.index+1 >= len(r.lines) {
		return 0, "", false
	}
	current := strings.TrimSpace(r.lines[r.index])
	next := strings.TrimSpace(r.lines[r.index+1])
	if current == "" {
		return 0, "", false
	}
	if isAll(next, '=') {
		return 1, current, true
	}
	if isAll(next, '-') {
		return 2, current, true
	}
	return 0, "", false
}

func (r *renderer) consumeFencedBlock(fence string) (string, int) {
	var body strings.Builder
	i := r.index + 1
	for i < len(r.lines) {
		line := r.lines[i]
		if strings.HasPrefix(strings.TrimSpace(line), fence) {
			return strings.TrimSuffix(body.String(), "\n"), i + 1
		}
		body.WriteString(line)
		body.WriteByte('\n')
		i++
	}
	return strings.TrimSuffix(body.String(), "\n"), i
}

func (r *renderer) writeFencedBlock(output *strings.Builder, info, body string) {
	language := parseFenceLanguage(info)
	if strings.EqualFold(language, "mermaid") {
		fmt.Fprintf(output, "<ac:structured-macro ac:name=\"%s\">\n", html.EscapeString(r.opts.MermaidMacroName))
		output.WriteString("<ac:plain-text-body><![CDATA[")
		output.WriteString(escapeCDATA(body))
		output.WriteString("]]></ac:plain-text-body>\n")
		output.WriteString("</ac:structured-macro>\n")
		return
	}

	fmt.Fprintf(output, "<ac:structured-macro ac:name=\"%s\">\n", html.EscapeString(r.opts.CodeMacroName))
	if language != "" {
		fmt.Fprintf(output, "<ac:parameter ac:name=\"language\">%s</ac:parameter>\n", html.EscapeString(language))
	}
	output.WriteString("<ac:plain-text-body><![CDATA[")
	output.WriteString(escapeCDATA(body))
	output.WriteString("]]></ac:plain-text-body>\n")
	output.WriteString("</ac:structured-macro>\n")
}

func (r *renderer) writeParagraph(output *strings.Builder) {
	var lines []string
	for r.index < len(r.lines) {
		line := r.lines[r.index]
		if strings.TrimSpace(line) == "" {
			break
		}
		if _, _, ok := parseFenceStart(line); ok {
			break
		}
		if isTableHeader(r.lines, r.index) {
			break
		}
		if matchesOrderedList(line) || matchesUnorderedList(line) {
			break
		}
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), ">") {
			break
		}
		if hrPattern.MatchString(strings.TrimSpace(line)) {
			break
		}
		if level, _ := parseATXHeading(line); level > 0 {
			break
		}
		if _, _, ok := r.parseSetextHeading(); ok {
			break
		}
		lines = append(lines, strings.TrimSpace(line))
		r.index++
	}

	if len(lines) == 0 {
		r.index++
		return
	}

	joined := strings.Join(lines, "\n")
	fmt.Fprintf(output, "<p>%s</p>\n", renderInline(joined))

	for r.index < len(r.lines) && strings.TrimSpace(r.lines[r.index]) == "" {
		r.index++
	}
}

func (r *renderer) writeList(output *strings.Builder) {
	ordered := matchesOrderedList(r.lines[r.index])
	tag := "ul"
	startValue := 1
	if ordered {
		tag = "ol"
		if m := orderedListPattern.FindStringSubmatch(r.lines[r.index]); len(m) > 1 {
			if value, err := strconv.Atoi(m[1]); err == nil {
				startValue = value
			}
		}
	}

	if ordered && startValue > 1 {
		fmt.Fprintf(output, "<%s start=\"%d\">\n", tag, startValue)
	} else {
		fmt.Fprintf(output, "<%s>\n", tag)
	}

	for r.index < len(r.lines) {
		line := r.lines[r.index]
		if strings.TrimSpace(line) == "" {
			break
		}

		if ordered {
			m := orderedListPattern.FindStringSubmatch(line)
			if len(m) == 0 {
				break
			}
			fmt.Fprintf(output, "<li>%s</li>\n", renderInline(strings.TrimSpace(m[2])))
		} else {
			m := unorderedListPattern.FindStringSubmatch(line)
			if len(m) == 0 {
				break
			}
			fmt.Fprintf(output, "<li>%s</li>\n", renderInline(strings.TrimSpace(m[1])))
		}
		r.index++
	}

	fmt.Fprintf(output, "</%s>\n", tag)
	for r.index < len(r.lines) && strings.TrimSpace(r.lines[r.index]) == "" {
		r.index++
	}
}

func (r *renderer) writeBlockquote(output *strings.Builder) {
	var lines []string
	for r.index < len(r.lines) {
		line := strings.TrimLeft(r.lines[r.index], " \t")
		if !strings.HasPrefix(line, ">") {
			break
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
		lines = append(lines, line)
		r.index++
	}
	content := strings.TrimSpace(strings.Join(lines, "\n"))
	if content == "" {
		return
	}
	fmt.Fprintf(output, "<blockquote><p>%s</p></blockquote>\n", renderInline(content))
	for r.index < len(r.lines) && strings.TrimSpace(r.lines[r.index]) == "" {
		r.index++
	}
}

func (r *renderer) writeTable(output *strings.Builder) {
	header := parseTableRow(r.lines[r.index])
	r.index += 2
	output.WriteString("<table>\n")
	output.WriteString("<tr>\n")
	for _, cell := range header {
		fmt.Fprintf(output, "<th>%s</th>\n", renderInline(cell))
	}
	output.WriteString("</tr>\n")

	for r.index < len(r.lines) {
		line := r.lines[r.index]
		if strings.TrimSpace(line) == "" || !strings.Contains(line, "|") {
			break
		}
		row := parseTableRow(line)
		if len(row) == 0 {
			break
		}
		output.WriteString("<tr>\n")
		for _, cell := range row {
			fmt.Fprintf(output, "<td>%s</td>\n", renderInline(cell))
		}
		output.WriteString("</tr>\n")
		r.index++
	}
	output.WriteString("</table>\n")
	for r.index < len(r.lines) && strings.TrimSpace(r.lines[r.index]) == "" {
		r.index++
	}
}

func parseATXHeading(line string) (int, string) {
	matches := atxHeadingPattern.FindStringSubmatch(line)
	if len(matches) != 3 {
		return 0, ""
	}
	return len(matches[1]), strings.TrimSpace(matches[2])
}

func parseFenceStart(line string) (fence, info string, ok bool) {
	matches := fenceStartPattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) != 3 {
		return "", "", false
	}
	return matches[1], strings.TrimSpace(matches[2]), true
}

func parseFenceLanguage(info string) string {
	if info == "" {
		return ""
	}
	fields := strings.Fields(info)
	if len(fields) == 0 {
		return ""
	}
	first := strings.TrimSpace(fields[0])
	if strings.ContainsAny(first, `:/\`) {
		return ""
	}
	if !fenceLanguagePattern.MatchString(first) {
		return ""
	}
	return strings.ToLower(first)
}

func splitLines(input string) []string {
	clean := strings.ReplaceAll(input, "\r\n", "\n")
	return strings.Split(clean, "\n")
}

func isAll(value string, target rune) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char != target {
			return false
		}
	}
	return true
}

func matchesOrderedList(line string) bool {
	return orderedListPattern.MatchString(line)
}

func matchesUnorderedList(line string) bool {
	return unorderedListPattern.MatchString(line)
}

func isTableHeader(lines []string, index int) bool {
	if index+1 >= len(lines) {
		return false
	}
	current := lines[index]
	next := lines[index+1]
	return strings.Contains(current, "|") && tableSepPattern.MatchString(strings.TrimSpace(next))
}

func parseTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "|") {
		trimmed = strings.TrimPrefix(trimmed, "|")
	}
	if strings.HasSuffix(trimmed, "|") {
		trimmed = strings.TrimSuffix(trimmed, "|")
	}
	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func escapeCDATA(raw string) string {
	return strings.ReplaceAll(raw, "]]>", "]]]]><![CDATA[>")
}

func renderInline(input string) string {
	var output strings.Builder
	for i := 0; i < len(input); {
		if input[i] == '`' {
			if end := strings.IndexByte(input[i+1:], '`'); end >= 0 {
				content := input[i+1 : i+1+end]
				output.WriteString("<code>")
				output.WriteString(html.EscapeString(content))
				output.WriteString("</code>")
				i += end + 2
				continue
			}
		}

		if strings.HasPrefix(input[i:], "**") {
			if end := strings.Index(input[i+2:], "**"); end >= 0 {
				content := input[i+2 : i+2+end]
				output.WriteString("<strong>")
				output.WriteString(renderInline(content))
				output.WriteString("</strong>")
				i += end + 4
				continue
			}
		}

		if strings.HasPrefix(input[i:], "~~") {
			if end := strings.Index(input[i+2:], "~~"); end >= 0 {
				content := input[i+2 : i+2+end]
				output.WriteString("<del>")
				output.WriteString(renderInline(content))
				output.WriteString("</del>")
				i += end + 4
				continue
			}
		}

		if input[i] == '*' {
			if end := strings.IndexByte(input[i+1:], '*'); end >= 0 {
				content := input[i+1 : i+1+end]
				output.WriteString("<em>")
				output.WriteString(renderInline(content))
				output.WriteString("</em>")
				i += end + 2
				continue
			}
		}

		if input[i] == '[' {
			if textEnd := strings.IndexByte(input[i+1:], ']'); textEnd >= 0 {
				label := input[i+1 : i+1+textEnd]
				restStart := i + 1 + textEnd + 1
				if restStart < len(input) && input[restStart] == '(' {
					if urlEnd := strings.IndexByte(input[restStart+1:], ')'); urlEnd >= 0 {
						url := input[restStart+1 : restStart+1+urlEnd]
						fmt.Fprintf(&output, "<a href=\"%s\">%s</a>", html.EscapeString(url), renderInline(label))
						i = restStart + urlEnd + 2
						continue
					}
				}
			}
		}

		if match := urlPattern.FindString(input[i:]); match != "" {
			fmt.Fprintf(&output, "<a href=\"%s\">%s</a>", html.EscapeString(match), html.EscapeString(match))
			i += len(match)
			continue
		}

		r, size := utf8.DecodeRuneInString(input[i:])
		if r == utf8.RuneError && size == 1 {
			output.WriteString(html.EscapeString(input[i : i+1]))
			i++
			continue
		}
		output.WriteString(html.EscapeString(string(r)))
		i += size
	}
	return output.String()
}
