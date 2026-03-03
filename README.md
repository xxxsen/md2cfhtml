# md2cfhtml

`md2cfhtml` 是一个将 Markdown 转换为 Confluence HTML（Storage Format）的 Go SDK，并提供了一个简易命令行工具 `conv`。

模块路径：`github.com/xxxsen/md2cfhtml`

## 作用

- 将普通 Markdown 内容转换为可粘贴/可写入 Confluence 的 HTML。
- 针对常见 Confluence 场景做了专门处理：
  - `[TOC]` -> `toc` macro
  - ```` ```mermaid ```` -> `mermaid-macro` macro
  - ```` ```plantuml ```` / ```` ```puml ```` -> `plantuml` macro
  - 其他代码块 -> `code` macro（自动带 `language` 参数）

## 安装

### 1. 安装命令行工具 `conv`

```bash
go install github.com/xxxsen/md2cfhtml/cmd/conv@latest
```

安装后可直接使用：

```bash
conv --input ./sample/test.md --output ./out.html
```

### 2. 在本地仓库中构建

```bash
go build -o conv ./cmd/conv
./conv --input ./sample/test.md --output ./out.html
```

### 3. 安装 SDK 依赖

```bash
go get github.com/xxxsen/md2cfhtml
```

## CLI 使用方式

命令：`conv`

参数：

- `--input`：输入 Markdown 文件路径（string，必填）
- `--output`：输出 HTML 文件路径（string，必填）

示例：

```bash
conv --input ./sample/test.md --output ./sample/test.html
```

## SDK 使用方式

### 1. 字符串转换

````go
package main

import (
	"fmt"

	"github.com/xxxsen/md2cfhtml"
)

func main() {
	md := `# Demo

[TOC]

```mermaid
flowchart TD
A --> B
```
`

	html, err := md2cfhtml.ConvertString(md)
	if err != nil {
		panic(err)
	}
	fmt.Println(html)
}
````

### 2. 文件转换

```go
package main

import "github.com/xxxsen/md2cfhtml"

func main() {
	err := md2cfhtml.ConvertFile("./sample/test.md", "./sample/test.html")
	if err != nil {
		panic(err)
	}
}
```

### 3. 可选配置

默认情况下不需要传任何 option，直接调用：

```go
html, err := md2cfhtml.Convert([]byte(markdown))
```

只有当你要覆盖默认宏名或关闭 TOC 转换时，才传 option：

```go
html, err := md2cfhtml.Convert([]byte(markdown),
	md2cfhtml.WithTOCMacroName("toc"),
	md2cfhtml.WithCodeMacroName("code"),
	md2cfhtml.WithMermaidMacroName("mermaid-macro"),
	md2cfhtml.WithPlantUMLMacroName("plantuml"),
	md2cfhtml.WithTOCMacroEnabled(true),
)
```

## 输入/输出说明

- 输入建议使用 UTF-8 编码的 Markdown 文本。
- 输出是 Confluence 可识别的 HTML 片段（含 `ac:structured-macro`）。
