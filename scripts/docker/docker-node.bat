@ECHO OFF

SET SELF=%0
SET DOCKER_IMAGE=harmonyone/node:s3

WHERE docker >NUL 2>&1
IF %ERRORLEVEL% NEQ 0 (
	ECHO docker is not installed.
	echo Please check https://docs.docker.com/docker-for-windows/install/ to get docker installed.
	GOTO :EOF
)

SET port_base=
SET kill_only=

SET account_id=%1
SHIFT

:loop
IF NOT "%1"=="" (
    IF "%1"=="-p" (
        SET port_base=%2
        SHIFT
    )
    IF "%1"=="-k" (
        SET kill_only=true
    )
	SHIFT
    GOTO :loop
)

IF "%account_id%"=="" (
	ECHO Please provide account id
	CALL :usage
	GOTO :EOF
)

IF "%port_base%"=="" SET port_base=9000
IF %port_base% LSS 4000 (
    ECHO port base cannot be less than 4000
	GOTO :EOF
)
IF %port_base% GTR 59900 (
    ECHO port base cannot be greater than 59900
	GOTO :EOF
)

SET /A port_rest="%port_base%-3000"
SET /A port_rpc="%port_base%+5555"

SET running_container=
FOR /F "tokens=* USEBACKQ" %%F IN (`docker ps -q -a -f "name=^harmony-%account_id%-%port_base%$"`) DO (
	SET running_container=%%F
)

IF NOT "%running_container%"=="" (
    ECHO Stop node for account id: %account_id% port %port_base%
	docker rm -v -f harmony-%account_id%-%port_base% >NUL
)

IF "%kill_only%"=="true" GOTO :EOF

echo Pull latest node image
docker pull %DOCKER_IMAGE% >NUL

docker run -it -d ^
	--name harmony-%account_id%-%port_base% ^
	-p %port_base%:%port_base% -p %port_rest%:%port_rest% -p %port_rpc%:%port_rpc% ^
	-e NODE_PORT=%port_base% ^
	-e NODE_ACCOUNT_ID=%account_id% ^
	--mount type=volume,source=db-%account_id%-%port_base%,destination=/harmony/db ^
	--mount type=volume,source=log-%account_id%-%port_base%,destination=/harmony/log ^
	%DOCKER_IMAGE% >NUL

echo.
echo ======================================
echo Node for account %account_id% (port %port_base%) is running in container 'harmony-%account_id%-%port_base%'"
echo.
echo To check console log, please run `docker logs -f harmony-%account_id%-%port_base%`"
echo To stop node, please run `%SELF% %account_id% -k -p %port_base%`"
echo ======================================"

GOTO :EOF

:usage
SETLOCAL
	ECHO usage: %SELF% account_id [-p base_port] [-k]
	ECHO   -p base_port: base port, default; 9000
	ECHO   -k          : kill running node
ENDLOCAL
