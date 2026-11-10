package vkgen

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Registry struct {
	Types      []Type      `xml:"types>type"`
	Enums      []Enums     `xml:"enums"`
	Commands   []Command   `xml:"commands>command"`
	Features   []Feature   `xml:"feature"`
	Extensions []Extension `xml:"extensions>extension"`
}

type Type struct {
	Category   string   `xml:"category,attr"`
	Name       string   `xml:"name,attr"`
	Alias      string   `xml:"alias,attr"`
	API        string   `xml:"api,attr"`
	Requires   string   `xml:"requires,attr"`
	BitValues  string   `xml:"bitvalues,attr"`
	Inner      string   `xml:",innerxml"`
	Members    []Member `xml:"member"`
	NameInside string   `xml:"name"`
	ProtoName  string   `xml:"proto>name"`
}

type Member struct {
	Values   string `xml:"values,attr"`
	Optional string `xml:"optional,attr"`
	Len      string `xml:"len,attr"`
	API      string `xml:"api,attr"`
	Inner    string `xml:",innerxml"`
}

type Enums struct {
	Name     string `xml:"name,attr"`
	Type     string `xml:"type,attr"`
	BitWidth string `xml:"bitwidth,attr"`
	Values   []Enum `xml:"enum"`
}

type Enum struct {
	Name      string `xml:"name,attr"`
	Value     string `xml:"value,attr"`
	BitPos    string `xml:"bitpos,attr"`
	Alias     string `xml:"alias,attr"`
	API       string `xml:"api,attr"`
	Type      string `xml:"type,attr"`
	Extends   string `xml:"extends,attr"`
	Offset    string `xml:"offset,attr"`
	ExtNumber string `xml:"extnumber,attr"`
	Dir       string `xml:"dir,attr"`
}

type Command struct {
	Name   string  `xml:"name,attr"`
	Alias  string  `xml:"alias,attr"`
	API    string  `xml:"api,attr"`
	Proto  Member  `xml:"proto"`
	Params []Param `xml:"param"`
}

type Param struct {
	Optional string `xml:"optional,attr"`
	Len      string `xml:"len,attr"`
	API      string `xml:"api,attr"`
	Inner    string `xml:",innerxml"`
}

type Feature struct {
	API      string    `xml:"api,attr"`
	Name     string    `xml:"name,attr"`
	Requires []Require `xml:"require"`
}

type Extension struct {
	Name      string    `xml:"name,attr"`
	Number    string    `xml:"number,attr"`
	Supported string    `xml:"supported,attr"`
	Requires  []Require `xml:"require"`
}

type Require struct {
	API   string `xml:"api,attr"`
	Enums []Enum `xml:"enum"`
}

func Parse(data []byte) (*Registry, error) {
	var r Registry
	if err := xml.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	if len(r.Types) == 0 || len(r.Commands) == 0 {
		return nil, fmt.Errorf("not a Vulkan registry: no types or commands")
	}
	return &r, nil
}

func forVulkan(api string) bool {
	if api == "" {
		return true
	}
	for _, a := range strings.Split(api, ",") {
		if a == "vulkan" {
			return true
		}
	}
	return false
}

type Decl struct {
	Const    bool
	Type     string
	Stars    int
	Name     string
	Dims     []string
	BitField bool
}

var (
	comments = regexp.MustCompile(`(?s)<comment>.*?</comment>`)
	typeTag  = regexp.MustCompile(`<type[^>]*>(.*?)</type>`)
	nameTag  = regexp.MustCompile(`<name[^>]*>(.*?)</name>`)
	dims     = regexp.MustCompile(`\[(?:<enum>(.*?)</enum>|(\d+))\]`)
)

func ParseDecl(inner string) (Decl, error) {
	text := comments.ReplaceAllString(inner, "")
	t := typeTag.FindStringSubmatchIndex(text)
	n := nameTag.FindStringSubmatchIndex(text)
	if t == nil || n == nil {
		return Decl{}, fmt.Errorf("cannot parse declaration %q", inner)
	}
	d := Decl{Type: text[t[2]:t[3]], Name: text[n[2]:n[3]]}
	d.Const = strings.Contains(text[:t[0]], "const")
	d.Stars = strings.Count(text[t[1]:n[0]], "*")
	rest := text[n[1]:]
	d.BitField = strings.HasPrefix(strings.TrimSpace(rest), ":")
	for _, m := range dims.FindAllStringSubmatch(rest, -1) {
		if m[1] != "" {
			d.Dims = append(d.Dims, m[1])
		} else {
			d.Dims = append(d.Dims, m[2])
		}
	}
	return d, nil
}

func enumValue(e Enum, extension int) (int64, bool, error) {
	switch {
	case e.Alias != "":
		return 0, false, nil
	case e.BitPos != "":
		pos, err := strconv.Atoi(e.BitPos)
		return int64(1) << pos, true, err
	case e.Offset != "":
		ext := extension
		if e.ExtNumber != "" {
			n, err := strconv.Atoi(e.ExtNumber)
			if err != nil {
				return 0, false, err
			}
			ext = n
		}
		offset, err := strconv.ParseInt(e.Offset, 10, 64)
		if err != nil {
			return 0, false, err
		}
		v := 1000000000 + int64(ext-1)*1000 + offset
		if e.Dir == "-" {
			v = -v
		}
		return v, true, nil
	case e.Value != "":
		v, err := parseNumber(e.Value)
		return v, err == nil, err
	}
	return 0, false, nil
}

func parseNumber(text string) (int64, error) {
	s := strings.Trim(text, "()")
	invert := strings.HasPrefix(s, "~")
	s = strings.TrimPrefix(s, "~")
	for _, suffix := range []string{"ULL", "LL", "UL", "U", "L"} {
		if strings.HasSuffix(s, suffix) {
			s = strings.TrimSuffix(s, suffix)
			break
		}
	}
	if strings.Contains(s, ".") || strings.HasPrefix(s, "\"") {
		return 0, fmt.Errorf("not an integer: %s", text)
	}
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		u, uerr := strconv.ParseUint(s, 0, 64)
		if uerr != nil {
			return 0, err
		}
		v = int64(u)
	}
	if invert {
		v = ^v
	}
	return v, nil
}
