@echo off
echo ==========================================
echo          Deploy to GitHub Repository  
echo ==========================================
echo.

set REPO_URL=https://github.com/baoyudeyu/9ybot.git

if not exist ".git" (
    echo [INFO] Initializing Git repository...
    git init
    if errorlevel 1 (
        echo [ERROR] Git initialization failed
        pause
        exit /b 1
    )
)

git remote get-url origin >nul 2>&1
if errorlevel 1 (
    echo [INFO] Adding remote repository...
    git remote add origin %REPO_URL%
) else (
    echo [INFO] Updating remote repository URL...
    git remote set-url origin %REPO_URL%
)

for /f "tokens=*" %%i in ('git branch --show-current 2^>nul') do set CURRENT_BRANCH=%%i
if "%CURRENT_BRANCH%"=="" (
    echo [INFO] Creating main branch...
    git checkout -b main
)

echo [INFO] Checking for git lock files...
if exist ".git\index.lock" (
    echo [WARNING] Removing git lock file...
    del ".git\index.lock" 2>nul
)
if exist ".git\HEAD.lock" (
    echo [WARNING] Removing HEAD lock file...
    del ".git\HEAD.lock" 2>nul
)

echo [INFO] Ensuring .gitignore exists...
if not exist ".gitignore" (
    echo Creating .gitignore file...
    echo node_modules/ > .gitignore
    echo *.log >> .gitignore
    echo .env >> .gitignore
    echo __pycache__/ >> .gitignore
    echo *.pyc >> .gitignore
) else (
    findstr /i "node_modules" .gitignore >nul 2>&1
    if errorlevel 1 (
        echo Adding node_modules to .gitignore...
        echo node_modules/ >> .gitignore
    )
)

echo [INFO] Adding files to staging area...
echo [INFO] Excluding node_modules directory...
git add . --ignore-errors
git reset HEAD node_modules/ 2>nul
git reset HEAD user/node_modules/ 2>nul
git reset HEAD backend/node_modules/ 2>nul

git diff --cached --quiet
if not errorlevel 1 (
    echo [WARNING] No changes detected
    echo [INFO] Creating initial commit...
    git commit --allow-empty -m "Initial commit: %date% %time%"
) else (
    echo [INFO] Committing changes...
    git commit -m "Auto deploy: %date% %time%"
)

echo [INFO] Checking if main branch exists...
git show-ref --verify --quiet refs/heads/main
if errorlevel 1 (
    echo [INFO] Creating main branch...
    git branch -M main
)

echo [INFO] Pushing to GitHub repository...
git push -u origin main --force

if errorlevel 1 (
    echo.
    echo [ERROR] Push failed. Possible reasons:
    echo 1. Network connection issues
    echo 2. Repository permission issues  
    echo 3. Git authentication issues
    echo.
    echo Please check:
    echo - GitHub repository exists and has write permission
    echo - Git username and email are configured
    echo - SSH key or personal access token configured
    echo.
    pause
    exit /b 1
)

echo.
echo ==========================================
echo          Deploy Success!
echo ==========================================
echo Repository: %REPO_URL%
echo Time: %date% %time%
echo ==========================================
echo.

pause 