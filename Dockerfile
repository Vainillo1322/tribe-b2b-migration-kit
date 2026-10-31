# Fusion: the migration-kit site (static), the Python Google inventory, and
# the gdrive2proton web wizard, served by one Go binary on $PORT.
FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/app .

FROM python:3.12-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY site /app/site
COPY inventory /app/inventory
COPY inventory/requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt
COPY --from=build /out/app /usr/local/bin/gdrive2proton
ENV PORT=3000 DATA_DIR=/data
WORKDIR /app
VOLUME /data
EXPOSE 3000
CMD ["gdrive2proton"]
