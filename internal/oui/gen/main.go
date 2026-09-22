// 从 Wireshark 的 manuf 文件生成精简的 MAC 厂商库（oui.tsv.gz）。
//
//	curl -L -o manuf https://www.wireshark.org/download/automated/data/manuf
//	go run ./internal/oui/gen manuf internal/oui/oui.tsv.gz
package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// 去掉公司名尾部的法律实体后缀，界面上显示更干净
var suffix = regexp.MustCompile(`(?i)[\s,.]*(co\.?,?\s*ltd\.?|company limited|corporation|corp\.?|incorporated|inc\.?|limited|ltd\.?|llc|l\.l\.c\.|gmbh|s\.?a\.?|ag|b\.?v\.?|pte\.?|pty\.?|plc|oy|ab|a/s|s\.?r\.?l\.?|technology|technologies|electronics|communication|communications|international)$`)

func clean(name string) string {
	name = strings.TrimSpace(name)
	for i := 0; i < 4; i++ {
		n := strings.TrimRight(suffix.ReplaceAllString(name, ""), " ,.")
		if n == name || len(n) < 3 {
			break
		}
		name = n
	}
	return name
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "用法: gen <manuf> <输出.tsv.gz>")
		os.Exit(2)
	}
	in, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer in.Close()
	type rec struct{ prefix, name string }
	var recs []rec
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		f := strings.Split(line, "\t")
		for i := range f {
			f[i] = strings.TrimSpace(f[i]) // 各列用空格补齐了宽度
		}
		if len(f) < 2 || strings.Contains(f[0], "/") || len(f[0]) != 8 {
			continue // 只保留 24 位前缀（MA-L）
		}
		name := f[1]
		if len(f) >= 3 && f[2] != "" {
			name = f[2]
		}
		recs = append(recs, rec{strings.ToUpper(strings.ReplaceAll(f[0], ":", "")), clean(name)})
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].prefix < recs[j].prefix })
	out, err := os.Create(os.Args[2])
	if err != nil {
		panic(err)
	}
	defer out.Close()
	zw, _ := gzip.NewWriterLevel(out, gzip.BestCompression)
	for _, r := range recs {
		fmt.Fprintf(zw, "%s\t%s\n", r.prefix, r.name)
	}
	zw.Close()
	fmt.Printf("写入 %d 条\n", len(recs))
}
