from flask import Flask, request

app = Flask(__name__)

@app.route("/", defaults={"path": ""}, methods=["GET", "POST"])
@app.route("/<path:path>", methods=["GET", "POST"])
def handle_request(path):
    print(f"Received request on mock service function at /{path}")
    return f"Mock function received your request at /{path}!", 200

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8082)
