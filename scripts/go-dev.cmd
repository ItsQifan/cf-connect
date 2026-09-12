@echo off
rem ---------------------------------------------------------------------------
rem Run Go for this repository, resolving the toolchain and module proxy.
rem
rem This wrapper exists because a Windows daemon/CI shell often does not inherit
rem the environment a developer set up interactively. It:
rem   1. locates GOROOT (GOROOT env var, then the conventional install paths)
rem   2. defaults GOPROXY to a mirror that works behind a restricted network
rem   3. passes every argument straight through to `go`
rem
rem Override anything by setting the variable before calling, e.g.
rem   set GOPROXY=direct && scripts\go-dev.cmd build ./...
rem ---------------------------------------------------------------------------
setlocal

if "%GOROOT%"=="" (
    if exist "%LOCALAPPDATA%\Programs\go-toolchain\go\bin\go.exe" (
        set "GOROOT=%LOCALAPPDATA%\Programs\go-toolchain\go"
    ) else if exist "%ProgramFiles%\Go\bin\go.exe" (
        set "GOROOT=%ProgramFiles%\Go"
    )
)

if "%GOPATH%"=="" set "GOPATH=%USERPROFILE%\go"

rem Only prepend to PATH when we actually resolved a GOROOT; otherwise rely on
rem the caller's PATH so a system-wide Go installation still works.
if not "%GOROOT%"=="" set "PATH=%GOROOT%\bin;%GOPATH%\bin;%PATH%"

rem A module proxy is required on networks that cannot reach proxy.golang.org.
if "%GOPROXY%"=="" set "GOPROXY=https://goproxy.cn,direct"

where go >nul 2>nul
if errorlevel 1 (
    echo go-dev: the Go toolchain was not found. >&2
    echo go-dev: install Go 1.25+ or set GOROOT to an existing installation. >&2
    exit /b 1
)

go %*
exit /b %ERRORLEVEL%
