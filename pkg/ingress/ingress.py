from flask import Flask, request, Response
import requests
import os

app = Flask(__name__)

@app.route("/", defaults={"path": ""}, methods=["GET", "POST", "PUT", "DELETE"])
@app.route("/<path:path>", methods=["GET", "POST", "PUT", "DELETE"])
def forward(path):
    target_host = os.getenv("TARGET_HOST", "localhost")
    target_port = os.getenv("TARGET_PORT", "80")
    target_url = f"http://{target_host}:{target_port}/{path}"
    method = request.method
    headers = {key: value for key, value in request.headers if key.lower() != 'host'}
    data = request.get_data()

    print(f"Forwarding {method} request to {target_url}")

    try:
        resp = requests.request(method, target_url, headers=headers, data=data, allow_redirects=False)
        return Response(resp.content, status=resp.status_code, headers=dict(resp.headers))
    except Exception as e:
        print(f"Error forwarding request: {e}")
        return Response(f"Error forwarding request to {target_host}:{target_port}", status=502)

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080)
