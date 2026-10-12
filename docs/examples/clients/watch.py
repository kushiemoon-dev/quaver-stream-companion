# Prints every event of the companion. Run with: python watch.py
import http.client
import json
import time
import urllib.request

URL = "http://127.0.0.1:7878/events"


def watch():
    data = []
    with urllib.request.urlopen(URL) as res:
        for raw in res:
            line = raw.decode("utf-8").rstrip("\r\n")
            if line.startswith("data: "):
                data.append(line[6:])
            elif line == "" and data:
                event = json.loads("\n".join(data))
                data = []
                print(event["type"], event["data"])


while True:
    try:
        watch()
    except (OSError, http.client.HTTPException, ValueError) as err:
        print("Disconnected:", err)
    time.sleep(2)
