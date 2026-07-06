@echo off
echo Installing InsightFace dependencies...
echo.

REM Buat virtual environment
python -m venv venv
echo Virtual environment created.

REM Aktifkan venv
call venv\Scripts\activate

REM Install semua library
pip install -r requirements.txt

echo.
echo Installation complete!
echo Jalankan start.bat untuk start service.
pause