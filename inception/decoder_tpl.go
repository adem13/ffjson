/**
 *  Copyright 2014 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package ffjsoninception

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"
)

var decodeTpl map[string]*template.Template

func init() {
	decodeTpl = make(map[string]*template.Template)

	funcs := map[string]string{
		"handlerNumeric":    handlerNumericTxt,
		"allowTokens":       allowTokensTxt,
		"handleFallback":    handleFallbackTxt,
		"handleString":      handleStringTxt,
		"handleObject":      handleObjectTxt,
		"handleArray":       handleArrayTxt,
		"handleSlice":       handleSliceTxt,
		"handleByteSlice":   handleByteSliceTxt,
		"handleBool":        handleBoolTxt,
		"handlePtr":         handlePtrTxt,
		"header":            headerTxt,
		"ujFunc":            ujFuncTxt,
		"handleUnmarshaler": handleUnmarshalerTxt,
	}

	tplFuncs := template.FuncMap{
		"getAllowTokens":      getAllowTokens,
		"getNumberSize":       getNumberSize,
		"getType":             getType,
		"handleField":         handleField,
		"handleFieldAddr":     handleFieldAddr,
		"unquoteField":        unquoteField,
		"getTmpVarFor":        getTmpVarFor,
		"getSetFieldMarkFunc": getSetFieldMarkFunc,
		"getFieldType":        getFieldType,
	}

	for k, v := range funcs {
		decodeTpl[k] = template.Must(template.New(k).Funcs(tplFuncs).Parse(v))
	}
}

func autoImport(ic *Inception, typ reflect.Type) {
	s := getFieldType(typ)

	switch s {
	case "time.Time":
		ic.OutputImports[`"time"`] = true
	case "tp.Datetime":
		ic.OutputImports[`"gitee.com/ystech/go-component/tp"`] = true
	}
}

func getFieldType(typ reflect.Type) string {
	return fmt.Sprintf("%v", typ)
}

func getSetFieldMarkFunc(name string) string {
	ns := strings.Split(name, ".")
	if len(ns) != 2 {
		return ""
	}

	return ns[0] + `.SetFieldMark("` + ns[1] + `")`
}

type handlerNumeric struct {
	IC        *Inception
	Name      string
	JsonName  string
	ParseFunc string
	Typ       reflect.Type
	TakeAddr  bool
}

var handlerNumericTxt = `
{
	{{$ic := .IC}}

	if tok == fflib.FFTok_null {
		{{if eq .TakeAddr true}}
		{{.Name}} = nil
		{{end}}
	} else {
		{{if eq .ParseFunc "ParseFloat" }}
		tval, err := fflib.{{ .ParseFunc}}(fs.Output.Bytes(), {{getNumberSize .Typ}})
		{{else}}
		tval, err := fflib.{{ .ParseFunc}}(fs.Output.Bytes(), 10, {{getNumberSize .Typ}})
		{{end}}

		if err != nil {
			//return fs.WrapErr(err)
			return errors.New({{.JsonName}} + "格式错误")
		}
		{{if eq .TakeAddr true}}
		ttypval := {{getType $ic .Name .Typ}}(tval)
		{{.Name}} = &ttypval
		{{else}}
		{{.Name}} = {{getType $ic .Name .Typ}}(tval)
		{{end}}
		
		//handlerNumericTxt
		{{getSetFieldMarkFunc .Name}}
	}
}
`

type allowTokens struct {
	Name     string
	JsonName string
	Tokens   []string
}

var allowTokensTxt = `
{
	if {{range $index, $element := .Tokens}}{{if ne $index 0 }}&&{{end}} tok != fflib.{{$element}}{{end}} {
		//return fs.WrapErr(fmt.Errorf("cannot unmarshal %s into Go value for {{.Name}}", tok))
		return errors.New({{.JsonName}} + "格式错误")
	}
}
`

type handleFallback struct {
	Name     string
	JsonName string
	Typ      reflect.Type
	Kind     reflect.Kind
}

var handleFallbackTxt = `
{
	/* Falling back. type={{printf "%v" .Typ}} kind={{printf "%v" .Kind}} */
	tbuf, err := fs.CaptureField(tok)
	if err != nil {
		//return fs.WrapErr(err)
		return errors.New({{.JsonName}} + "格式错误")
	}

	err = json.Unmarshal(tbuf, &{{.Name}})
	if err != nil {
		//return fs.WrapErr(err)
		return errors.New({{.JsonName}} + "格式错误")
	}

	//handleFallbackTxt
	{{getSetFieldMarkFunc .Name}}
}
`

type handleString struct {
	IC       *Inception
	Name     string
	JsonName string
	Typ      reflect.Type
	TakeAddr bool
	Quoted   bool
}

var handleStringTxt = `
{
	{{$ic := .IC}}

	{{getAllowTokens .Typ.Name .JsonName "FFTok_string" "FFTok_null"}}
	if tok == fflib.FFTok_null {
	{{if eq .TakeAddr true}}
		{{.Name}} = nil
	{{end}}
	} else {
	{{if eq .TakeAddr true}}
		var tval {{getType $ic .Name .Typ}}
		outBuf := fs.Output.Bytes()
		{{unquoteField .Quoted}}
		tval = {{getType $ic .Name .Typ}}(string(outBuf))
		{{.Name}} = &tval
	{{else}}
		outBuf := fs.Output.Bytes()
		{{unquoteField .Quoted}}
		{{.Name}} = {{getType $ic .Name .Typ}}(string(outBuf))
	{{end}}

	//handleStringTxt
	{{getSetFieldMarkFunc .Name}}
	}
}
`

type handleObject struct {
	IC       *Inception
	Name     string
	JsonName string
	Typ      reflect.Type
	Ptr      reflect.Kind
	TakeAddr bool
}

var handleObjectTxt = `
{
	{{$ic := .IC}}
	{{getAllowTokens .Typ.Name .JsonName "FFTok_left_bracket" "FFTok_null"}}
	if tok == fflib.FFTok_null {
		{{.Name}} = nil
	} else {

		{{if eq .TakeAddr true}}
			{{if eq .Typ.Elem.Kind .Ptr }}
				{{if eq .Typ.Key.Kind .Ptr }}
				var tval = make(map[*{{getType $ic .Name .Typ.Key.Elem}}]*{{getType $ic .Name .Typ.Elem.Elem}}, 0)
				{{else}}
				var tval = make(map[{{getType $ic .Name .Typ.Key}}]*{{getType $ic .Name .Typ.Elem.Elem}}, 0)
				{{end}}
			{{else}}
				{{if eq .Typ.Key.Kind .Ptr }}
				var tval = make(map[*{{getType $ic .Name .Typ.Key.Elem}}]{{getType $ic .Name .Typ.Elem}}, 0)
				{{else}}
				var tval = make(map[{{getType $ic .Name .Typ.Key}}]{{getType $ic .Name .Typ.Elem}}, 0)
				{{end}}
			{{end}}
		{{else}}
			{{if eq .Typ.Elem.Kind .Ptr }}
				{{if eq .Typ.Key.Kind .Ptr }}
				{{.Name}} = make(map[*{{getType $ic .Name .Typ.Key.Elem}}]*{{getType $ic .Name .Typ.Elem.Elem}}, 0)
				{{else}}
				{{.Name}} = make(map[{{getType $ic .Name .Typ.Key}}]*{{getType $ic .Name .Typ.Elem.Elem}}, 0)
				{{end}}
			{{else}}
				{{if eq .Typ.Key.Kind .Ptr }}
				{{.Name}} = make(map[*{{getType $ic .Name .Typ.Key.Elem}}]{{getType $ic .Name .Typ.Elem}}, 0)
				{{else}}
				{{.Name}} = make(map[{{getType $ic .Name .Typ.Key}}]{{getType $ic .Name .Typ.Elem}}, 0)
				{{end}}
			{{end}}
		{{end}}

		wantVal := true

		for {
		{{$keyPtr := false}}
		{{if eq .Typ.Key.Kind .Ptr }}
			{{$keyPtr := true}}
			var k *{{getType $ic .Name .Typ.Key.Elem}}
		{{else}}
			var k {{getType $ic .Name .Typ.Key}}
		{{end}}

		{{$valPtr := false}}
		{{$tmpVar := getTmpVarFor .Name}}
		{{if eq .Typ.Elem.Kind .Ptr }}
			{{$valPtr := true}}
			var {{$tmpVar}} *{{getType $ic .Name .Typ.Elem.Elem}}
		{{else}}
			var {{$tmpVar}} {{getType $ic .Name .Typ.Elem}}
		{{end}}

			tok = fs.Scan()
			if tok == fflib.FFTok_error {
				goto tokerror
			}
			if tok == fflib.FFTok_right_bracket {
				break
			}

			if tok == fflib.FFTok_comma {
				if wantVal == true {
					// TODO(pquerna): this isn't an ideal error message, this handles
					// things like [,,,] as an array value.
					// return fs.WrapErr(fmt.Errorf("wanted value token, but got token: %v", tok))
					return errors.New({{.JsonName}} + "格式错误")
				}
				continue
			} else {
				wantVal = true
			}

			{{handleField .IC "k" .JsonName .Typ.Key $keyPtr false}}

			// Expect ':' after key
			tok = fs.Scan()
			if tok != fflib.FFTok_colon {
				// return fs.WrapErr(fmt.Errorf("wanted colon token, but got token: %v", tok))
				return errors.New({{.JsonName}} + "格式错误")
			}

			tok = fs.Scan()
			{{handleField .IC $tmpVar .JsonName .Typ.Elem $valPtr false}}

			{{if eq .TakeAddr true}}
			tval[k] = {{$tmpVar}}
			{{else}}
			{{.Name}}[k] = {{$tmpVar}}
			{{end}}
			wantVal = false
		}

		{{if eq .TakeAddr true}}
		{{.Name}} = &tval
		{{end}}

		//handleObjectTxt
		{{getSetFieldMarkFunc .Name}}
	}
}
`

type handleArray struct {
	IC              *Inception
	Name            string
	JsonName        string
	Typ             reflect.Type
	Ptr             reflect.Kind
	UseReflectToSet bool
	IsPtr           bool
}

var handleArrayTxt = `
{
	{{$ic := .IC}}
	{{getAllowTokens .Typ.Name .JsonName "FFTok_left_brace" "FFTok_null"}}
	{{if eq .Typ.Elem.Kind .Ptr}}
		{{.Name}} = [{{.Typ.Len}}]*{{getType $ic .Name .Typ.Elem.Elem}}{}
	{{else}}
		{{.Name}} = [{{.Typ.Len}}]{{getType $ic .Name .Typ.Elem}}{}
	{{end}}
	if tok != fflib.FFTok_null {
		wantVal := true

		idx := 0
		for {
			{{$ptr := false}}
			{{$tmpVar := getTmpVarFor .Name}}
			{{if eq .Typ.Elem.Kind .Ptr }}
				{{$ptr := true}}
				var {{$tmpVar}} *{{getType $ic .Name .Typ.Elem.Elem}}
			{{else}}
				var {{$tmpVar}} {{getType $ic .Name .Typ.Elem}}
			{{end}}

			tok = fs.Scan()
			if tok == fflib.FFTok_error {
				goto tokerror
			}
			if tok == fflib.FFTok_right_brace {
				break
			}

			if tok == fflib.FFTok_comma {
				if wantVal == true {
					// TODO(pquerna): this isn't an ideal error message, this handles
					// things like [,,,] as an array value.
					// return fs.WrapErr(fmt.Errorf("wanted value token, but got token: %v", tok))
					return errors.New({{.JsonName}} + "格式错误")
				}
				continue
			} else {
				wantVal = true
			}

			{{handleField .IC $tmpVar .JsonName .Typ.Elem $ptr false}}

			// Standard json.Unmarshal ignores elements out of array bounds,
			// that what we do as well.
			if idx < {{.Typ.Len}} {
				{{.Name}}[idx] = {{$tmpVar}}
				idx++
			}

			wantVal = false
		}

		//handleArrayTxt
		{{getSetFieldMarkFunc .Name}}
	}
}
`

var handleSliceTxt = `
{
	{{$ic := .IC}}
	{{getAllowTokens .Typ.Name .JsonName "FFTok_left_brace" "FFTok_null"}}
	if tok == fflib.FFTok_null {
		{{.Name}} = nil
	} else {
		{{if eq .Typ.Elem.Kind .Ptr }}
			{{if eq .IsPtr true}}
				{{.Name}} = &[]*{{getType $ic .Name .Typ.Elem.Elem}}{}
			{{else}}
				{{.Name}} = []*{{getType $ic .Name .Typ.Elem.Elem}}{}
			{{end}}
		{{else}}
			{{if eq .IsPtr true}}
				{{.Name}} = &[]{{getType $ic .Name .Typ.Elem}}{}
			{{else}}
				{{.Name}} = []{{getType $ic .Name .Typ.Elem}}{}
			{{end}}
		{{end}}

		wantVal := true

		for {
			{{$ptr := false}}
			{{$tmpVar := getTmpVarFor .Name}}
			{{if eq .Typ.Elem.Kind .Ptr }}
				{{$ptr := true}}
				var {{$tmpVar}} *{{getType $ic .Name .Typ.Elem.Elem}}
			{{else}}
				var {{$tmpVar}} {{getType $ic .Name .Typ.Elem}}
			{{end}}

			tok = fs.Scan()
			if tok == fflib.FFTok_error {
				goto tokerror
			}
			if tok == fflib.FFTok_right_brace {
				break
			}

			if tok == fflib.FFTok_comma {
				if wantVal == true {
					// TODO(pquerna): this isn't an ideal error message, this handles
					// things like [,,,] as an array value.
					// return fs.WrapErr(fmt.Errorf("wanted value token, but got token: %v", tok))
					return errors.New({{.JsonName}} + "格式错误")
				}
				continue
			} else {
				wantVal = true
			}

			{{handleField .IC $tmpVar .JsonName .Typ.Elem $ptr false}}
			{{if eq .IsPtr true}}
				*{{.Name}} = append(*{{.Name}}, {{$tmpVar}})
			{{else}}
				{{.Name}} = append({{.Name}}, {{$tmpVar}})
			{{end}}
			wantVal = false
		}

		//handleSliceTxt
		{{getSetFieldMarkFunc .Name}}
	}
}
`

var handleByteSliceTxt = `
{
	{{getAllowTokens .Typ.Name .JsonName "FFTok_string" "FFTok_null"}}
	if tok == fflib.FFTok_null {
		{{.Name}} = nil
	} else {
		b := make([]byte, base64.StdEncoding.DecodedLen(fs.Output.Len()))
		n, err := base64.StdEncoding.Decode(b, fs.Output.Bytes())
		if err != nil {
			// return fs.WrapErr(err)
			return errors.New({{.JsonName}} + "格式错误")
		}
		{{if eq .UseReflectToSet true}}
			v := reflect.ValueOf(&{{.Name}}).Elem()
			v.SetBytes(b[0:n])
		{{else}}
			{{.Name}} = append([]byte(), b[0:n]...)
		{{end}}

		//handleByteSliceTxt
		{{getSetFieldMarkFunc .Name}}
	}
}
`

type handleBool struct {
	Name     string
	JsonName string
	Typ      reflect.Type
	TakeAddr bool
}

var handleBoolTxt = `
{
	if tok == fflib.FFTok_null {
		{{if eq .TakeAddr true}}
		{{.Name}} = nil
		{{end}}
	} else {
		tmpb := fs.Output.Bytes()

		{{if eq .TakeAddr true}}
		var tval bool
		{{end}}

		if bytes.Compare([]byte{'t', 'r', 'u', 'e'}, tmpb) == 0 {
		{{if eq .TakeAddr true}}
			tval = true
		{{else}}
			{{.Name}} = true
		{{end}}
		} else if bytes.Compare([]byte{'f', 'a', 'l', 's', 'e'}, tmpb) == 0 {
		{{if eq .TakeAddr true}}
			tval = false
		{{else}}
			{{.Name}} = false
		{{end}}
		} else {
			// err = errors.New("unexpected bytes for true/false value")
			// return fs.WrapErr(err)
			return errors.New({{.JsonName}} + "格式错误")
		}

		{{if eq .TakeAddr true}}
		{{.Name}} = &tval
		{{end}}

		//handleBoolTxt
		{{getSetFieldMarkFunc .Name}}
	}
}
`

type handlePtr struct {
	IC       *Inception
	Name     string
	JsonName string
	Typ      reflect.Type
	Quoted   bool
}

var handlePtrTxt = `
{
	{{$ic := .IC}}

	if tok == fflib.FFTok_null {
		{{.Name}} = nil
	} else {
		if {{.Name}} == nil {
			{{.Name}} = new({{getType $ic .Typ.Elem.Name .Typ.Elem}})
		}

		{{handleFieldAddr .IC .Name .JsonName true .Typ.Elem false .Quoted}}

		//handlePtrTxt
		{{getSetFieldMarkFunc .Name}}
	}
}
`

type header struct {
	IC *Inception
	SI *StructInfo
}

var headerTxt = `
const (
	ffj_t_{{.SI.Name}}base = iota
	ffj_t_{{.SI.Name}}no_such_key
	{{with $si := .SI}}
		{{range $index, $field := $si.Fields}}
			{{if ne $field.JsonName "-"}}
		ffj_t_{{$si.Name}}_{{$field.Name}}
			{{end}}
		{{end}}
	{{end}}
)

{{with $si := .SI}}
	{{range $index, $field := $si.Fields}}
		{{if ne $field.JsonName "-"}}
var ffj_key_{{$si.Name}}_{{$field.Name}} = []byte({{$field.JsonName}})
		{{end}}
	{{end}}
{{end}}

`

type ujFunc struct {
	IC          *Inception
	SI          *StructInfo
	ValidValues []string
	ResetFields bool
}

var ujFuncTxt = `
{{$si := .SI}}
{{$ic := .IC}}

func New{{.SI.Name}}() *{{.SI.Name}} {
	uj := &{{.SI.Name}}{}
	uj.ResetFieldMark()
	return uj
}

//NewItems 创建对应的切片指针对象，配合 go-component/orm 组件
func (uj *{{$.SI.Name}}) NewItems() interface{} {
	items := new([]{{$.SI.Name}})
	*items = make([]{{$.SI.Name}}, 0)
	return items
}

//FieldMarks 列出所有已赋值的字段名称列表
func (uj *{{$.SI.Name}}) FieldMarks() []string {
	names := make([]string, 0, len(uj.fieldMark))
	for k, v := range uj.fieldMark {
		if v {
			names = append(names, k)
		}
	}

	return names
}

//ResetFieldMark 重置所有字段的赋值标识为:false，字段内容并不会清空
func (uj *{{$.SI.Name}}) ResetFieldMark() {
	if uj.fieldMark == nil {
		uj.fieldMark = make(map[string]bool)
	}
	
	{{range $index, $field := $si.Fields}}
	uj.fieldMark["{{$field.Name}}"] = false
	{{end}}
}

//SetFieldMark 设置字段的赋值标识，isMark不传时，默认:true
func (uj *{{$.SI.Name}}) SetFieldMark(fieldName string, isMark ...bool) {
	if len(isMark) == 1 {
		uj.fieldMark[fieldName] = isMark[0]
		return
	}
	
	uj.fieldMark[fieldName] = true
}

{{range $index, $field := $si.Fields}}
//{{$field.Name}}Mark {{$field.Name}}是否已赋值（赋值标识）
func (uj *{{$.SI.Name}}) {{$field.Name}}Mark() bool {
	return uj.fieldMark["{{$field.Name}}"]
}

//Set{{$field.Name}} 设置{{$field.Name}}的值，并将赋值标识设为:true
func (uj *{{$.SI.Name}}) Set{{$field.Name}}(val {{getFieldType .Typ}}) {
	uj.{{$field.Name}} = val
	uj.SetFieldMark("{{$field.Name}}")
}
{{end}}

func (uj *{{$.SI.Name}}) autoSetFieldValue(jsonBytes *fflib.Buffer, typ, key, val string) {
	strType := "string,tp.Datetime,time.Time"

	if jsonBytes.Len() > 1 {
		jsonBytes.WriteByte(',')
	}

	jsonBytes.WriteByte('"')
	jsonBytes.WriteString(key)
	jsonBytes.WriteString("\":")

	if strings.Contains(strType, typ) {
		fflib.WriteJsonString(jsonBytes, val)
	} else {
		jsonBytes.WriteString(val)
	}
}

//AutoSetFieldValue 根据map自动设置字段值
func (uj *{{$.SI.Name}}) AutoSetFieldValue(pm map[string]string) error {
	if len(pm) == 0 {
		return nil
	}

	var jsonBytes fflib.Buffer
	jsonBytes.WriteByte('{')
	for k, v := range pm {
		fieldName := strings.ToLower(k)
		switch fieldName {
			{{range $index, $field := $si.Fields}}
			{{if ne $field.JsonName "-"}}
			case strings.ToLower({{$field.JsonName}}):
				uj.autoSetFieldValue(&jsonBytes, "{{getFieldType .Typ}}", {{$field.JsonName}}, v)
			{{end}}
			{{end}}
		}
	}
	jsonBytes.WriteByte('}')

	return uj.UnmarshalJSON(jsonBytes.Bytes())
}

func (uj *{{.SI.Name}}) UnmarshalJSON(input []byte) error {
	uj.ResetFieldMark()

	fs := fflib.NewFFLexer(input)
    return uj.UnmarshalJSONFFLexer(fs, fflib.FFParse_map_start)
}

func (uj *{{.SI.Name}}) UnmarshalJSONFFLexer(fs *fflib.FFLexer, state fflib.FFParseState) error {
	var err error = nil
	currentKey := ffj_t_{{.SI.Name}}base
	_ = currentKey
	tok := fflib.FFTok_init
	wantedTok := fflib.FFTok_init

				{{if eq .ResetFields true}}
				{{range $index, $field := $si.Fields}}
				var ffj_set_{{$si.Name}}_{{$field.Name}} = false
 				{{end}}
				{{end}}

mainparse:
	for {
		tok = fs.Scan()
		//	println(fmt.Sprintf("debug: tok: %v  state: %v", tok, state))
		if tok == fflib.FFTok_error {
			goto tokerror
		}

		switch state {

		case fflib.FFParse_map_start:
			if tok != fflib.FFTok_left_bracket {
				wantedTok = fflib.FFTok_left_bracket
				goto wrongtokenerror
			}
			state = fflib.FFParse_want_key
			continue

		case fflib.FFParse_after_value:
			if tok == fflib.FFTok_comma {
				state = fflib.FFParse_want_key
			} else if tok == fflib.FFTok_right_bracket {
				goto done
			} else {
				wantedTok = fflib.FFTok_comma
				goto wrongtokenerror
			}

		case fflib.FFParse_want_key:
			// json {} ended. goto exit. woo.
			if tok == fflib.FFTok_right_bracket {
				goto done
			}
			if tok != fflib.FFTok_string {
				wantedTok = fflib.FFTok_string
				goto wrongtokenerror
			}

			kn := fs.Output.Bytes()
			if len(kn) <= 0 {
				// "" case. hrm.
				currentKey = ffj_t_{{.SI.Name}}no_such_key
				state = fflib.FFParse_want_colon
				goto mainparse
			} else {
				switch kn[0] {
				{{range $byte, $fields := $si.FieldsByFirstByte}}
				case '{{$byte}}':
					{{range $index, $field := $fields}}
						{{if ne $index 0 }}} else if {{else}}if {{end}} bytes.Equal(ffj_key_{{$si.Name}}_{{$field.Name}}, kn) {
						currentKey = ffj_t_{{$si.Name}}_{{$field.Name}}
						state = fflib.FFParse_want_colon
						goto mainparse
					{{end}} }
				{{end}}
				}
				{{range $index, $field := $si.ReverseFields}}
				if {{$field.FoldFuncName}}(ffj_key_{{$si.Name}}_{{$field.Name}}, kn) {
					currentKey = ffj_t_{{$si.Name}}_{{$field.Name}}
					state = fflib.FFParse_want_colon
					goto mainparse
				}
				{{end}}
				currentKey = ffj_t_{{.SI.Name}}no_such_key
				state = fflib.FFParse_want_colon
				goto mainparse
			}

		case fflib.FFParse_want_colon:
			if tok != fflib.FFTok_colon {
				wantedTok = fflib.FFTok_colon
				goto wrongtokenerror
			}
			state = fflib.FFParse_want_value
			continue
		case fflib.FFParse_want_value:

			if {{range $index, $v := .ValidValues}}{{if ne $index 0 }}||{{end}}tok == fflib.{{$v}}{{end}} {
				switch currentKey {
				{{range $index, $field := $si.Fields}}
				case ffj_t_{{$si.Name}}_{{$field.Name}}:
					goto handle_{{$field.Name}}
				{{end}}
				case ffj_t_{{$si.Name}}no_such_key:
					err = fs.SkipField(tok)
					if err != nil {
						return fs.WrapErr(err)
					}
					state = fflib.FFParse_after_value
					goto mainparse
				}
			} else {
				goto wantedvalue
			}
		}
	}
{{range $index, $field := $si.Fields}}
handle_{{$field.Name}}:
	{{with $fieldName := $field.Name | printf "uj.%s"}}
		{{handleField $ic $fieldName $field.JsonName $field.Typ $field.Pointer $field.ForceString}}
		{{if eq $.ResetFields true}}
		ffj_set_{{$si.Name}}_{{$field.Name}} = true
		{{end}}
		state = fflib.FFParse_after_value
		goto mainparse
	{{end}}
{{end}}

wantedvalue:
	// return fs.WrapErr(fmt.Errorf("wanted value token, but got token: %v", tok))
	switch currentKey {
	{{range $index, $field := $si.Fields}}
	case ffj_t_{{$si.Name}}_{{$field.Name}}:
		return errors.New({{$field.JsonName}} + "格式错误")
	{{end}}
	}
wrongtokenerror:
	return fs.WrapErr(fmt.Errorf("ffjson: wanted token: %v, but got token: %v output=%s", wantedTok, tok, fs.Output.String()))
tokerror:
	if fs.BigError != nil {
		return fs.WrapErr(fs.BigError)
	}
	err = fs.Error.ToError()
	if err != nil {
		return fs.WrapErr(err)
	}
	panic("ffjson-generated: unreachable, please report bug.")
done:
{{if eq .ResetFields true}}
{{range $index, $field := $si.Fields}}
	if !ffj_set_{{$si.Name}}_{{$field.Name}} {
	{{with $fieldName := $field.Name | printf "uj.%s"}}
	{{if eq $field.Pointer true}}
		{{$fieldName}} = nil
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Interface), 10) + `}}
		{{$fieldName}} = nil
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Slice), 10) + `}}
		{{$fieldName}} = nil
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Array), 10) + `}}
		{{$fieldName}} = [{{$field.Typ.Len}}]{{getType $ic $fieldName $field.Typ.Elem}}{}
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Map), 10) + `}}
		{{$fieldName}} = nil
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Bool), 10) + `}}
		{{$fieldName}} = false
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.String), 10) + `}}
		{{$fieldName}} = ""
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Struct), 10) + `}}
		{{$fieldName}} = {{getType $ic $fieldName $field.Typ}}{}
	{{else}}
		{{$fieldName}} = {{getType $ic $fieldName $field.Typ}}(0)
	{{end}}
	{{end}}
	}
{{end}}
{{end}}
	return nil
}
`

type handleUnmarshaler struct {
	IC                   *Inception
	Name                 string
	JsonName             string
	Typ                  reflect.Type
	Ptr                  reflect.Kind
	TakeAddr             bool
	UnmarshalJSONFFLexer bool
	Unmarshaler          bool
}

var handleUnmarshalerTxt = `
	{{$ic := .IC}}

	{{if eq .UnmarshalJSONFFLexer true}}
	{
		if tok == fflib.FFTok_null {
				{{if eq .Typ.Kind .Ptr }}
					{{.Name}} = nil
				{{end}}
				{{if eq .TakeAddr true }}
					{{.Name}} = nil
				{{end}}
				state = fflib.FFParse_after_value
				goto mainparse
		}
		{{if eq .Typ.Kind .Ptr }}
			if {{.Name}} == nil {
				{{.Name}} = new({{getType $ic .Typ.Elem.Name .Typ.Elem}})
			}
		{{end}}
		{{if eq .TakeAddr true }}
			if {{.Name}} == nil {
				{{.Name}} = new({{getType $ic .Typ.Name .Typ}})
			}
		{{end}}
		err = {{.Name}}.UnmarshalJSONFFLexer(fs, fflib.FFParse_want_key)
		if err != nil {
			// return err
			return errors.New({{.JsonName}} + "格式错误")
		}
		state = fflib.FFParse_after_value

		//handleUnmarshalerTxt
		{{getSetFieldMarkFunc .Name}}
	}
	{{else}}
	{{if eq .Unmarshaler true}}
	{
		if tok == fflib.FFTok_null {
			{{if eq .TakeAddr true }}
				{{.Name}} = nil
			{{end}}
			state = fflib.FFParse_after_value
			goto mainparse
		}

		tbuf, err := fs.CaptureField(tok)
		if err != nil {
			// return fs.WrapErr(err)
			return errors.New({{.JsonName}} + "格式错误")
		}

		{{if eq .TakeAddr true }}
		if {{.Name}} == nil {
			{{.Name}} = new({{getType $ic .Typ.Name .Typ}})
		}
		{{end}}
		err = {{.Name}}.UnmarshalJSON(tbuf)
		if err != nil {
			// return fs.WrapErr(err)
			return errors.New({{.JsonName}} + "格式错误")
		}
		state = fflib.FFParse_after_value

		//handleUnmarshalerTxt
		{{getSetFieldMarkFunc .Name}}
	}
	{{end}}
	{{end}}
`
