from flask import Flask, request, Response
import requests
import os
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

@app.route("/", defaults={"path": ""}, methods=["GET", "POST", "PUT", "DELETE"])
@app.route("/<path:path>", methods=["GET", "POST", "PUT", "DELETE"])
def forward(path):
    # Extract environment variables
    target_host = os.getenv("TARGET_HOST", "localhost")
    target_port = os.getenv("TARGET_PORT", "80")
    app_host = os.getenv("APP_HOST", "localhost")
    app_port = os.getenv("APP_PORT", "81")
    
    method = request.method
    headers = {key: value for key, value in request.headers if key.lower() != 'host'}
    data = request.get_data()

    try:
        # Step 1: Forward to the app
        intermediate_url = f"http://{app_host}:{app_port}/{path}"
        logger.info(f"Forwarding {method} request to intermediate app at {intermediate_url}")
        intermediate_response = requests.request(method, intermediate_url, headers=headers, data=data, allow_redirects=False)

        # Step 2: Forward app's response to the final target
        target_url = f"http://{target_host}:{target_port}/{path}"
        logger.info(f"Forwarding response from app to final target at {target_url}")
        # Use response body from intermediate app as new request body
        final_response = requests.request(
            method, target_url,
            headers=headers,
            data=intermediate_response.content,
            allow_redirects=False
        )

        return Response(final_response.content, status=final_response.status_code, headers=dict(final_response.headers))

    except Exception as e:
        print(f"Error during forwarding chain: {e}")
        return Response(f"Error in forwarding chain to {target_url}", status=502)

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8082)
