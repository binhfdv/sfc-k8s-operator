from flask import Flask, request, Response
import requests
import os

app = Flask(__name__)

TARGET_HOST = os.getenv("TARGET_HOST", "localhost")
TARGET_PORT = os.getenv("TARGET_PORT", "80")

@app.route("/", defaults={"path": ""}, methods=["GET", "POST", "PUT", "DELETE"])
@app.route("/<path:path>", methods=["GET", "POST", "PUT", "DELETE"])
def forward(path):
    target_url = f"http://{TARGET_HOST}:{TARGET_PORT}/{path}"
    method = request.method
    headers = {key: value for key, value in request.headers if key.lower() != 'host'}
    data = request.get_data()

    print(f"Forwarding {method} request to {target_url}")

    try:
        resp = requests.request(method, target_url, headers=headers, data=data, allow_redirects=False)
        return Response(resp.content, status=resp.status_code, headers=dict(resp.headers))
    except Exception as e:
        print(f"Error forwarding request: {e}")
        return Response("Error forwarding request", status=502)

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8082)
