package md2cfhtml

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extensionast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

var fenceLanguagePattern = regexp.MustCompile(`^[A-Za-z0-9_+\-.#]+$`)

// Options controls how markdown is converted to Confluence HTML.
type Options struct {
	EnableTOCMacro    bool
	TOCMacroName      string
	CodeMacroName     string
	MermaidMacroName  string
	PlantUMLMacroName string
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

// WithMermaidMacroName overrides the mermaid macro name. Default is "mermaid-macro".
func WithMermaidMacroName(name string) Option {
	return func(o *Options) {
		if strings.TrimSpace(name) != "" {
			o.MermaidMacroName = strings.TrimSpace(name)
		}
	}
}

// WithPlantUMLMacroName overrides the PlantUML macro name. Default is "plantuml".
func WithPlantUMLMacroName(name string) Option {
	return func(o *Options) {
		if strings.TrimSpace(name) != "" {
			o.PlantUMLMacroName = strings.TrimSpace(name)
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
	markdown goldmark.Markdown
	opts     Options
}

func defaultOptions() Options {
	return Options{
		EnableTOCMacro:    true,
		TOCMacroName:      "toc",
		CodeMacroName:     "code",
		MermaidMacroName:  "mermaid-macro",
		PlantUMLMacroName: "plantuml",
	}
}

// NewConverter creates a converter with optional custom settings.
func NewConverter(options ...Option) *Converter {
	opts := defaultOptions()
	for _, apply := range options {
		apply(&opts)
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
	)

	return &Converter{
		markdown: md,
		opts:     opts,
	}
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
	document := c.markdown.Parser().Parse(text.NewReader(markdown))
	renderer := confluenceRenderer{
		source:         markdown,
		opts:           c.opts,
		skipParagraphs: map[ast.Node]bool{},
	}

	if err := renderer.render(document); err != nil {
		return nil, err
	}

	return renderer.buffer.Bytes(), nil
}

type confluenceRenderer struct {
	buffer         bytes.Buffer
	source         []byte
	opts           Options
	skipParagraphs map[ast.Node]bool
}

func (r *confluenceRenderer) render(document ast.Node) error {
	err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		switch typed := node.(type) {
		case *ast.Document:
			return ast.WalkContinue, nil
		case *ast.Heading:
			if entering {
				fmt.Fprintf(&r.buffer, "<h%d>", typed.Level)
			} else {
				fmt.Fprintf(&r.buffer, "</h%d>\n", typed.Level)
			}
			return ast.WalkContinue, nil
		case *ast.Paragraph:
			if entering {
				if r.opts.EnableTOCMacro && r.isTOCParagraph(typed) {
					fmt.Fprintf(&r.buffer, "<ac:structured-macro ac:name=\"%s\"></ac:structured-macro>\n", html.EscapeString(r.opts.TOCMacroName))
					r.skipParagraphs[node] = true
					return ast.WalkSkipChildren, nil
				}
				r.buffer.WriteString("<p>")
			} else {
				if r.skipParagraphs[node] {
					delete(r.skipParagraphs, node)
					return ast.WalkContinue, nil
				}
				r.buffer.WriteString("</p>\n")
			}
			return ast.WalkContinue, nil
		case *ast.Text:
			if !entering {
				return ast.WalkContinue, nil
			}
			r.writeEscaped(typed.Segment.Value(r.source))
			if typed.HardLineBreak() {
				r.buffer.WriteString("<br />")
			} else if typed.SoftLineBreak() {
				r.buffer.WriteByte('\n')
			}
			return ast.WalkContinue, nil
		case *ast.String:
			if entering {
				r.writeEscaped(typed.Value)
			}
			return ast.WalkContinue, nil
		case *ast.Emphasis:
			if typed.Level >= 2 {
				if entering {
					r.buffer.WriteString("<strong>")
				} else {
					r.buffer.WriteString("</strong>")
				}
				return ast.WalkContinue, nil
			}
			if entering {
				r.buffer.WriteString("<em>")
			} else {
				r.buffer.WriteString("</em>")
			}
			return ast.WalkContinue, nil
		case *extensionast.Strikethrough:
			if entering {
				r.buffer.WriteString("<del>")
			} else {
				r.buffer.WriteString("</del>")
			}
			return ast.WalkContinue, nil
		case *ast.Blockquote:
			if entering {
				r.buffer.WriteString("<blockquote>\n")
			} else {
				r.buffer.WriteString("</blockquote>\n")
			}
			return ast.WalkContinue, nil
		case *ast.List:
			if entering {
				if typed.IsOrdered() {
					if typed.Start > 1 {
						fmt.Fprintf(&r.buffer, "<ol start=\"%d\">\n", typed.Start)
					} else {
						r.buffer.WriteString("<ol>\n")
					}
				} else {
					r.buffer.WriteString("<ul>\n")
				}
			} else {
				if typed.IsOrdered() {
					r.buffer.WriteString("</ol>\n")
				} else {
					r.buffer.WriteString("</ul>\n")
				}
			}
			return ast.WalkContinue, nil
		case *ast.ListItem:
			if entering {
				r.buffer.WriteString("<li>")
			} else {
				r.buffer.WriteString("</li>\n")
			}
			return ast.WalkContinue, nil
		case *ast.Link:
			if entering {
				fmt.Fprintf(&r.buffer, "<a href=\"%s\">", html.EscapeString(string(typed.Destination)))
			} else {
				r.buffer.WriteString("</a>")
			}
			return ast.WalkContinue, nil
		case *ast.AutoLink:
			if !entering {
				return ast.WalkContinue, nil
			}
			href := html.EscapeString(string(typed.URL(r.source)))
			label := href
			if labelBytes := typed.Label(r.source); len(labelBytes) > 0 {
				label = html.EscapeString(string(labelBytes))
			}
			fmt.Fprintf(&r.buffer, "<a href=\"%s\">%s</a>", href, label)
			return ast.WalkSkipChildren, nil
		case *ast.Image:
			if !entering {
				return ast.WalkContinue, nil
			}
			alt := html.EscapeString(strings.TrimSpace(r.plainText(node)))
			src := html.EscapeString(string(typed.Destination))
			if alt == "" {
				fmt.Fprintf(&r.buffer, "<img src=\"%s\" />", src)
			} else {
				fmt.Fprintf(&r.buffer, "<img src=\"%s\" alt=\"%s\" />", src, alt)
			}
			return ast.WalkSkipChildren, nil
		case *ast.CodeSpan:
			if !entering {
				return ast.WalkContinue, nil
			}
			r.buffer.WriteString("<code>")
			r.writeEscaped(typed.Text(r.source))
			r.buffer.WriteString("</code>")
			return ast.WalkSkipChildren, nil
		case *ast.FencedCodeBlock:
			if !entering {
				return ast.WalkContinue, nil
			}
			language := parseFenceLanguage(typed.Info.Text(r.source))
			content := r.linesText(typed.Lines())
			if strings.EqualFold(language, "mermaid") {
				r.renderMermaidMacro(content)
			} else if isPlantUMLLanguage(language) {
				r.renderPlantUMLMacro(content)
			} else {
				r.renderCodeMacro(language, content)
			}
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock:
			if !entering {
				return ast.WalkContinue, nil
			}
			r.renderCodeMacro("", r.linesText(typed.Lines()))
			return ast.WalkSkipChildren, nil
		case *ast.ThematicBreak:
			if entering {
				r.buffer.WriteString("<hr />\n")
			}
			return ast.WalkSkipChildren, nil
		case *extensionast.Table:
			if entering {
				r.buffer.WriteString("<table>\n")
			} else {
				r.buffer.WriteString("</table>\n")
			}
			return ast.WalkContinue, nil
		case *extensionast.TableHeader:
			if entering {
				r.buffer.WriteString("<thead>\n")
			} else {
				r.buffer.WriteString("</thead>\n")
			}
			return ast.WalkContinue, nil
		case *extensionast.TableRow:
			if entering {
				r.buffer.WriteString("<tr>\n")
			} else {
				r.buffer.WriteString("</tr>\n")
			}
			return ast.WalkContinue, nil
		case *extensionast.TableCell:
			if entering {
				if r.isHeaderCell(node) {
					r.buffer.WriteString("<th>")
				} else {
					r.buffer.WriteString("<td>")
				}
			} else {
				if r.isHeaderCell(node) {
					r.buffer.WriteString("</th>\n")
				} else {
					r.buffer.WriteString("</td>\n")
				}
			}
			return ast.WalkContinue, nil
		case *ast.HTMLBlock:
			if !entering {
				return ast.WalkContinue, nil
			}
			r.buffer.Write(typed.Text(r.source))
			r.buffer.WriteByte('\n')
			return ast.WalkSkipChildren, nil
		case *ast.RawHTML:
			if !entering {
				return ast.WalkContinue, nil
			}
			r.buffer.Write(typed.Segments.Value(r.source))
			return ast.WalkSkipChildren, nil
		default:
			return ast.WalkContinue, nil
		}
	})
	if err != nil {
		return fmt.Errorf("render markdown: %w", err)
	}
	return nil
}

func (r *confluenceRenderer) renderCodeMacro(language, content string) {
	fmt.Fprintf(&r.buffer, "<ac:structured-macro ac:name=\"%s\">\n", html.EscapeString(r.opts.CodeMacroName))
	if language != "" {
		fmt.Fprintf(&r.buffer, "<ac:parameter ac:name=\"language\">%s</ac:parameter>\n", html.EscapeString(language))
	}
	r.buffer.WriteString("<ac:plain-text-body><![CDATA[")
	r.buffer.WriteString(escapeCDATA(content))
	r.buffer.WriteString("]]></ac:plain-text-body>\n")
	r.buffer.WriteString("</ac:structured-macro>\n")
}

func (r *confluenceRenderer) renderMermaidMacro(content string) {
	fmt.Fprintf(&r.buffer, "<ac:structured-macro ac:name=\"%s\">\n", html.EscapeString(r.opts.MermaidMacroName))
	r.buffer.WriteString("<ac:plain-text-body><![CDATA[")
	r.buffer.WriteString(escapeCDATA(content))
	r.buffer.WriteString("]]></ac:plain-text-body>\n")
	r.buffer.WriteString("</ac:structured-macro>\n")
}

func (r *confluenceRenderer) renderPlantUMLMacro(content string) {
	fmt.Fprintf(&r.buffer, "<ac:structured-macro ac:name=\"%s\">\n", html.EscapeString(r.opts.PlantUMLMacroName))
	r.buffer.WriteString("<ac:plain-text-body><![CDATA[")
	r.buffer.WriteString(escapeCDATA(content))
	r.buffer.WriteString("]]></ac:plain-text-body>\n")
	r.buffer.WriteString("</ac:structured-macro>\n")
}

func (r *confluenceRenderer) linesText(lines *text.Segments) string {
	var content strings.Builder
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		content.Write(segment.Value(r.source))
	}
	return strings.TrimSuffix(content.String(), "\n")
}

func (r *confluenceRenderer) writeEscaped(raw []byte) {
	r.buffer.WriteString(html.EscapeString(string(raw)))
}

func (r *confluenceRenderer) plainText(node ast.Node) string {
	var content strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch typed := child.(type) {
		case *ast.Text:
			content.Write(typed.Segment.Value(r.source))
		case *ast.String:
			content.Write(typed.Value)
		default:
			content.WriteString(r.plainText(child))
		}
	}
	return content.String()
}

func (r *confluenceRenderer) isTOCParagraph(paragraph *ast.Paragraph) bool {
	return strings.EqualFold(strings.TrimSpace(r.plainText(paragraph)), "[TOC]")
}

func (r *confluenceRenderer) isHeaderCell(node ast.Node) bool {
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		if _, ok := parent.(*extensionast.TableHeader); ok {
			return true
		}
	}
	return false
}

func parseFenceLanguage(info []byte) string {
	trimmed := strings.TrimSpace(string(info))
	if trimmed == "" {
		return ""
	}
	fields := strings.Fields(trimmed)
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

func isPlantUMLLanguage(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "plantuml", "puml":
		return true
	default:
		return false
	}
}

func escapeCDATA(raw string) string {
	return strings.ReplaceAll(raw, "]]>", "]]]]><![CDATA[>")
}
