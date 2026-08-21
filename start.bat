@echo off
setlocal EnableExtensions

cd /d "%~dp0"
set "BOT_PORT=%ADMIN_PORT%"
if "%BOT_PORT%"=="" set "BOT_PORT=3007"

where go >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Go 1.23+ was not found.
  pause
  exit /b 1
)

if "%FARM_MASTER_KEY%"=="" (
  echo [ERROR] FARM_MASTER_KEY is required; refusing to start without credential encryption.
  pause
  exit /b 1
)

if not exist "%~dp0assets\webdist\index.html" (
  where pnpm >nul 2>nul
  if errorlevel 1 (
    where corepack >nul 2>nul
    if errorlevel 1 (
      echo [ERROR] pnpm or corepack is required to build the web assets.
      pause
      exit /b 1
    )
    call corepack pnpm install -r
    call corepack pnpm -C web build
  ) else (
    call pnpm install -r
    call pnpm -C web build
  )
  if errorlevel 1 exit /b 1
)

echo [INFO] FarmBot panel: http://localhost:%BOT_PORT%
set "ADMIN_PORT=%BOT_PORT%"
go run ./cmd/farmbot
