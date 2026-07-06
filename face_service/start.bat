@echo off
echo Starting InsightFace Service...
echo.

REM Aktifkan virtual environment
call venv\Scripts\activate

REM Jalankan service
uvicorn main:app --host 0.0.0.0 --port 8001 --reload

pause