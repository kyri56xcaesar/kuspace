FROM python:3.12-slim
RUN apt update && pip install boto3
COPY bash_app.py /bash_app.py
ENTRYPOINT [ "python3", "/bash_app.py" ]