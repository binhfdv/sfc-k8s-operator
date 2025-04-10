FROM python:3.10-slim

WORKDIR /app

COPY . .

RUN pip install -r requirements.txt

ENV TARGET_HOST=localhost
ENV TARGET_PORT=80

CMD ["python3", "ingress.py"]
