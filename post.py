import socket


def encode(s: str) -> str:
    res = ""
    for c in s:
        if not c.isascii():
            for b in c.encode("utf-8"):
                res += f"%{b:02X}"
        else:
            res += c
    return res


dct = {
    "ETJ6wOXggr8z": "чЗДШzЦеhфАйцБглРКг2IпЬШ1yZБк6Ъo5Pю5hшAвPЬ",
    "Qr1t1nOPmhdn0bP": "qЙцъйгеLЯм9tёUфnСьH6uвHT",
    "I5Yw09luKJR8fjW2": "VфdАFыНSsяUх2ХЬРмГUб0ЖkфCКв",
    "MCYm8DAELaS7": "SCБj2хьХ6кT79аряУiДРEКbRSQxЕЫCa",
    "Cae0kAAdDvtHrZlc": "ЛсЪЯЁмЛЮ0bмYьpеИITкнйжйСфtTXФGНвРтrGQоблl",
}
pairs = [f"{encode(k)}={encode(v)}" for k, v in dct.items()]
body = "&".join(pairs).encode()
ln = len(body)

headers = [
    "POST /yjyOQlwtYQR HTTP/1.1",
    "Host: hw1.alexbers.com",
    "Cookie: user=955ce0824bff0ae7b7a01eb55cb62e9b",
    "Content-Type: application/x-www-form-urlencoded",
    f"Content-Length: {ln}",
    "Connection: close",
]
head = ("\r\n".join(headers) + "\r\n\r\n").encode()
with socket.create_connection(("hw1.alexbers.com", 80)) as s:
    s.sendall(head + body)
    response = b""
    while True:
        chunk = s.recv(4096)
        if not chunk:
            break
        response += chunk

print(response.decode())
