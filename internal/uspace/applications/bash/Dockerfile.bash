# build context: internal/uspace/applications (shares common/kuspace_io.py)
FROM python:3.12-slim
COPY common/kuspace_io.py bash/bash_app.py /
ENTRYPOINT [ "python3", "/bash_app.py" ]