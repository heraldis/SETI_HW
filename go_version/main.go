package main

import (
	"fmt"
	"html"
	"io"
	"net"
	"regexp"
	"strings"
	"time"
)

type key_value struct {
	key   string
	value string
}

type request_data struct {
	method  string
	path    string
	headers []key_value
	query   []key_value
	cookies []key_value
	form    []key_value
	files   []key_value
	body    []byte
	ctype   string
}

const host string = "hw1.alexbers.com"
const user string = "user=955ce0824bff0ae7b7a01eb55cb62e9b"

const td_pattern string = `<td><code>(.*?)</code></td>`

var tr_pattern = regexp.MustCompile(fmt.Sprintf(`(?si)<tr>\s*%v\s*%v\s*</tr>`, td_pattern, td_pattern))

var actions = []string{
	"Отправьте POST-запрос по адресу",
	"Отправьте GET-запрос по адресу",
	"Загрузите файлы по адресу",
	"Перейдите по",
}

var sections = map[string]string{
	"Запрос должен иметь следующие заголовки:":                                 "headers",
	"При переходе выставьте следующие параметры запроса, указанные в таблице:": "query",
	"В запросе должны быть выставлены cookie:":                                 "cookies",
	"Запрос должен иметь следующие данные формы:":                              "form",
	"Загрузите файлы по адресу":                                                "files",
}

func encode(s string) string {
	res := ""
	for i := range s {
		b := s[i]
		if b > 127 {
			res += fmt.Sprintf("%%%02X", b)
		} else {
			res += string(b)
		}
	}
	return res
}

func extract_table_pairs(start int, page string) []key_value {
	end := strings.Index(page[start:], "</table>") + start
	pairs := tr_pattern.FindAllStringSubmatch(page[start:end], -1)
	res := []key_value{}
	for _, pair := range pairs {
		k := html.UnescapeString(strings.TrimSpace(pair[1]))
		v := html.UnescapeString(strings.TrimSpace(pair[2]))
		if !strings.Contains(k, "<th>") {
			s := key_value{key: k, value: v}
			res = append(res, s)
		}
	}

	return res
}

func send_start_request() []byte {
	lines := []string{
		"GET / HTTP/1.1",
		fmt.Sprintf("Host: %s", host),
		fmt.Sprintf("Cookie: %s", user),
		"Connection: close",
	}
	return []byte(strings.Join(lines, "\r\n") + "\r\n\r\n")
}

func send_request(req request_data) []byte {
	path := req.path

	if len(req.query) > 0 {
		parts := []string{}
		for _, pair := range req.query {
			parts = append(parts, encode(pair.key)+"="+encode(pair.value))
		}
		path += "?" + strings.Join(parts, "&")
	}

	headers := []string{
		fmt.Sprintf("%s %s HTTP/1.1", req.method, path),
		fmt.Sprintf("Host: %s", host),
	}
	for _, pair := range req.headers {
		headers = append(headers, fmt.Sprintf("%s: %s", pair.key, pair.value))
	}

	cookies := []string{fmt.Sprintf("%s", user)}
	for _, pair := range req.cookies {
		cookies = append(cookies, fmt.Sprintf("%s=%s", pair.key, pair.value))
	}
	headers = append(headers, "Cookie: "+strings.Join(cookies, "; "))

	if len(req.body) > 0 {
		headers = append(headers, fmt.Sprintf("Content-Length: %d", len(req.body)))
		if len(req.ctype) > 0 {
			headers = append(headers, fmt.Sprintf("Content-Type: %s", req.ctype))
		}
	}
	headers = append(headers, "Connection: close")

	return append([]byte(strings.Join(headers, "\r\n")+"\r\n\r\n"), req.body...)
}

func get_response(s net.Conn) string {
	s.SetDeadline(time.Now().Add(100 * time.Second))

	response, err := io.ReadAll(s)
	if err != nil {
		fmt.Printf("Critical error: %v", err)
	}

	fmt.Println(string(response))
	return string(response)
}

func parse_html(page string) request_data {
	req := request_data{
		path: "/",
	}

	action := ""
	for _, a := range actions {
		if strings.Contains(page, a) {
			action = a
			break
		}
	}
	if action == "" {
		fmt.Println("No action found")
		return request_data{}
	}

	action_idx := strings.Index(page, action)

	if strings.Contains(action, "POST") || strings.Contains(action, "файлы") {
		req.method = "POST"
	} else {
		req.method = "GET"
	}

	code_pttrn := regexp.MustCompile(`(?s)<code>(/[^<]*)</code>`)
	href_pttrn := regexp.MustCompile(`(?s)href=["']([^"']+)["']`)
	if address := code_pttrn.FindStringSubmatch(page[action_idx:]); address != nil {
		req.path = address[1]
	} else if address := href_pttrn.FindStringSubmatch(page[action_idx:]); address != nil {
		req.path = address[1]
	}
	if req.path != "" {
		req.path = html.UnescapeString(strings.TrimSpace(req.path))
	}

	for sectn, field := range sections {
		start := strings.Index(page[action_idx:], sectn)
		if start != -1 {
			pairs := extract_table_pairs(start+action_idx, page)

			switch field {
			case "headers":
				req.headers = pairs
			case "query":
				req.query = pairs
			case "cookies":
				req.cookies = pairs
			case "form":
				req.form = pairs
			case "files":
				req.files = pairs
			}
		}
	}

	if len(req.form) > 0 {
		form_parts := []string{}
		for _, pair := range req.form {
			form_parts = append(form_parts, encode(pair.key)+"="+encode(pair.value))
		}
		req.body = []byte(strings.Join(form_parts, "&"))
		req.ctype = "application/x-www-form-urlencoded"
	} else if len(req.files) > 0 {
		boundary := "----randomBoundary"
		parts := []byte{}

		for _, pair := range req.files {
			s := ("--" + boundary + "\r\n" +
				fmt.Sprintf("Content-Disposition: form-data; name=\"%[1]s\"; filename=\"%[1]s\"\r\n", pair.key) +
				"Content-Type: application/octet-stream`" +
				pair.value + "\r\n")
			parts = append(parts, []byte(s)...)
		}
		parts = append(parts, fmt.Appendf(nil, `--%s--\r\n`, boundary)...)
		req.body = parts
		req.ctype = fmt.Sprintf("multipart/form-data; boundary=%s", boundary)
	}

	return req
}

func request(request_bytes []byte) (string, error) {
	s, err := net.Dial("tcp", host+":80")
	if err != nil {
		return "", err
	}
	defer s.Close()

	_, werr := s.Write(request_bytes)
	if werr != nil {
		return "", werr
	}

	page := get_response(s)
	return page, nil
}

func main() {
	fmt.Println("Start request")
	start_tx := send_start_request()
	page, err := request(start_tx)
	if err != nil {
		fmt.Printf("Critical error on start: %v", err)
	}
	fmt.Println("First page fetched")

	for {
		fmt.Println("Page fetched")
		if strings.TrimSpace(page) == "" {
			fmt.Println("Page is empty")
			break
		}

		data := parse_html(page)
		if len(data.method) == 0 {
			fmt.Println("Fetching pages interrupted")
			break
		}

		raw_tx := send_request(data)
		page, err = request(raw_tx)
		if err != nil {
			fmt.Printf("Critical error: %v", err)
			break
		}

	}
}
