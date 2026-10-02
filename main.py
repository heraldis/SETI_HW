import html
import re
import socket

HOST = "hw1.alexbers.com"
USER = "user=955ce0824bff0ae7b7a01eb55cb62e9b"

ACTIONS = [
    "Отправьте POST-запрос по адресу",
    "Отправьте GET-запрос по адресу",
    "Загрузите файлы по адресу",
    "Перейдите по",
]

SECTIONS = {
    "Запрос должен иметь следующие заголовки:": "headers",
    "При переходе выставьте следующие параметры запроса, указанные в таблице:": "query",
    "В запросе должны быть выставлены cookie:": "cookies",
    "Запрос должен иметь следующие данные формы:": "form",
    "Загрузите файлы по адресу": "files",
}


def encode(s: str) -> str:
    res = ""
    for c in s:
        if not c.isascii():
            for b in c.encode("utf-8"):
                res += f"%{b:02X}"
        else:
            res += c
    return res


def extract_table_pairs(start_pos: int, tr_pattern: re.Pattern, page: str):
    end_pos = page.find("</table>", start_pos)
    pairs = tr_pattern.findall(page[start_pos:end_pos])
    return [
        (html.unescape(k.strip()), html.unescape(v.strip()))
        for k, v in pairs
        if "<th>" not in k
    ]


def send_start_request() -> bytes:
    lines = [
            "GET / HTTP/1.1",
            f"Host: {HOST}",
            f"Cookie: {USER}",
            # "Connection: close",
        ]
    return ("\r\n".join(lines) + "\r\n\r\n").encode()


def send_request(req: dict):
    path = req["path"]

    if req["query"]:
        q_str = "&".join(f"{encode(k)}={encode(v)}" for k, v in req["query"])
        path += f"?{q_str}"

    headers = [f"{req['method']} {path} HTTP/1.1", f"Host: {HOST}"]
    for k, v in req["headers"]:
        headers.append(f"{k}: {v}")

    cookies_list = [USER]
    if req["cookies"]:
        for k, v in req["cookies"]:
            cookies_list.append(f"{k}={v}")
    headers.append(f"Cookie: {'; '.join(cookies_list)}")

    if req["body"]:
        headers.append(f"Content-Length: {len(req['body'])}")
        if req["ctype"]:
            headers.append(f"Content-Type: {req['ctype']}")
    # headers.append("Connection: close")

    request_bytes = "\r\n".join(headers).encode("utf-8") + b"\r\n\r\n" + req["body"]
    return request_bytes


def get_response(s: socket.SocketType) -> str:
    # s.settimeout(100)
    response = b""
    # try:
    while True:
        chunk = s.recv(4096)
        if not chunk:
            break
        response += chunk
    # except socket.timeout:
    #     print("Timeout error")

    print(response.decode(errors="ignore"))
    return response.decode(errors="ignore")


def parse_html(page: str) -> dict:
    req = {
        "method": "GET",
        "path": "/",
        "headers": [],
        "query": [],
        "cookies": [],
        "form": [],
        "files": [],
        "body": b"",
        "ctype": None,
    }

    # action
    action = ""
    for actn in ACTIONS:
        if actn in page:
            action = actn
            break
    if len(action) == 0:
        print("No action found")
        return {}

    action_index = page.find(action)

    # method
    req["method"] = "POST" if "POST" in action or "файлы" in action else "GET"

    # page link
    address = re.search(r"<code>(/[^<]*)</code>", page[action_index:]) or re.search(
        r"""href=["']([^"']+)["']""", page[action_index:]
    )
    if address:
        req["path"] = html.unescape(address.group(1)).strip()

    # page patterns
    td_pattern = r"<td><code>(.*?)</code></td>"
    tr_pattern = re.compile(rf"<tr>{td_pattern}{td_pattern}</tr>", re.S | re.I)

    # data to insert
    for sectn, field in SECTIONS.items():
        start = page.find(sectn)
        if start != -1:
            req[field] = extract_table_pairs(start, tr_pattern, page)

    if req["form"]:
        req["body"] = "&".join(
            f"{encode(k)}={encode(v)}" for k, v in req["form"]
        ).encode("utf-8")
        req["ctype"] = "application/x-www-form-urlencoded"

    elif req["files"]:
        boundary = "----randomBoundary"
        parts = []
        for name, content in req["files"]:
            parts.append(
                (
                    f"--{boundary}\r\n"
                    f'Content-Disposition: form-data; name="{name}"; filename="{name}"\r\n'
                    f"Content-Type: application/octet-stream\r\n\r\n"
                ).encode("utf-8")
                + content.encode("utf-8")
                + b"\r\n"
            )
        parts.append(f"--{boundary}--\r\n".encode("utf-8"))
        req["body"] = b"".join(parts)
        req["ctype"] = f"multipart/form-data; boundary={boundary}"

    return req

print("Start request")
with socket.create_connection(("hw1.alexbers.com", 80)) as s:
    print("Start connected")
    s.sendall(send_start_request())
    print("Start sent")
    page = get_response(s)
print("First page fetched")

while True:
    print("Page fetched")
    if not page.strip():
        print("Empty page")
        break
    data = parse_html(page)
    if not data:
        print("Fetching pages interrupted. Last response:")
        print(ascii(page))
        break
    raw_tx = send_request(data)
    with socket.create_connection((HOST, 80)) as s:
        s.sendall(raw_tx)
        print("Request sent")
        page = get_response(s)
