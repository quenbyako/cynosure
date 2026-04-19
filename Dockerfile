# syntax=docker/dockerfile:1

FROM golang:1-bookworm AS build

ARG TARGETOS
ARG TARGETARCH

ARG PRIMARY_APP_NAME="cynosure"
ARG APP_VERSION="snapshot"
ARG APP_COMMIT="unknown"
ARG APP_DATE="0001-01-01T00:00:00Z"

ENV GOPRIVATE=github.com/quenbyako/*
ENV CGO_ENABLED=0

WORKDIR /src

# 1. Подготовка кэша зависимостей
# Копируем go.mod/sum и директорию contrib, так как она нужна для go mod download из-за replace
COPY go.mod go.sum ./
COPY contrib/ contrib/

# Скачиваем зависимости с использованием секретов.
# --mount=type=secret позволяет безопасно пробросить токен без сохранения в слоях.
# --mount=type=cache сохраняет скачанные модули локально.
RUN --mount=type=secret,id=GITHUB_TOKEN \
    --mount=type=cache,target=/go/pkg/mod \
    if [ -f /run/secrets/GITHUB_TOKEN ]; then \
        GITHUB_TOKEN_VAL=$(cat /run/secrets/GITHUB_TOKEN); \
        git config --global url."https://x-access-token:${GITHUB_TOKEN_VAL}@github.com/".insteadOf "https://github.com/"; \
    fi && \
    go mod download

# 2. Сборка приложения
COPY . .

# Собираем бинарник. Используем кэш для компилятора чтобы ускорить пересборку
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -mod=readonly -o /bin/app \
        -ldflags="-s -w \
            -X 'main.version=${APP_VERSION}' \
            -X 'main.commit=${APP_COMMIT}' \
            -X 'main.date=${APP_DATE}' \
            -X 'google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'" \
        -v \
        ./cmd/${PRIMARY_APP_NAME}

# 3. Финальный образ
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /bin/app /bin/app

ENTRYPOINT ["/bin/app"]
