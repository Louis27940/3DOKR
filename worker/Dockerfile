FROM mcr.microsoft.com/dotnet/sdk:10.0.404-alpine3.24 AS builder

WORKDIR /src

COPY Worker.csproj ./

RUN dotnet restore 

COPY . ./ 

RUN dotnet publish -c Release -o /src/publish --self-contained false --no-restore

FROM mcr.microsoft.com/dotnet/runtime:10.0.15-alpine3.24-amd64 AS runner

WORKDIR /app

COPY --from=builder --chown=app:app /src/publish/ .

USER app

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD test -f /tmp/healthy || exit 1

CMD ["dotnet", "Worker.dll"]

