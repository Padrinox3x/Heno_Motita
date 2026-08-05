<#
.SYNOPSIS
    Prueba integral de la API Heno Motita: salud -> login -> manager -> crew ->
    registro de alumnos (batch) -> activacion de cuenta -> envio de codigo por correo.

.DESCRIPTION
    Ejecuta el flujo completo contra la API para comprobar que las APIs
    funcionan, incluido el envio del codigo de activacion por correo (SendGrid).

    Si el -TestEmail ya esta registrado en el sistema, el script NO falla:
    detecta al alumno existente y usa POST /students/:id/new-activation-code
    para generar un codigo nuevo y enviarlo por correo a esa direccion.

.PARAMETER BaseUrl
    URL base de la API. Por defecto http://localhost:8080

.PARAMETER AdminEmail
    Correo del super administrador. Por defecto admin@henomotita.com

.PARAMETER AdminPassword
    Contrasena del super administrador. Por defecto Admin1234

.PARAMETER TestEmail
    Correo real donde quieres recibir el codigo de activacion
    (p. ej. tu Gmail). Si se omite, se usan correos de prueba.

.PARAMETER SkipActivation
    Si esta presente, no ejecuta la activacion de la cuenta del alumno.

.EXAMPLE
    .\test-api.ps1

.EXAMPLE
    .\test-api.ps1 -TestEmail picazoaranzoloomar@gmail.com

.EXAMPLE
    .\test-api.ps1 -BaseUrl https://tu-api.onrender.com -AdminEmail admin@correo.com -AdminPassword "MiPass"
#>
[CmdletBinding()]
param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$AdminEmail = "admin@henomotita.com",
    [string]$AdminPassword = "Admin1234",
    [string]$TestEmail = "",
    [switch]$SkipActivation
)

$ErrorActionPreference = "Stop"

# =============================================
# HELPERS
# =============================================

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Path,
        [hashtable]$Headers = @{},
        [string]$Body = ""
    )

    $uri = "$BaseUrl$Path"
    $params = @{
        Uri             = $uri
        Method          = $Method
        Headers         = $Headers
        ContentType     = "application/json"
        UseBasicParsing = $true
        ErrorAction     = "Stop"
    }
    if ($Body -ne "") { $params.Body = $Body }

    try {
        $resp = Invoke-WebRequest @params
        $data = $null
        if ($resp.Content) {
            try { $data = $resp.Content | ConvertFrom-Json } catch { $data = $resp.Content }
        }
        return [pscustomobject]@{
            Ok     = $true
            Status = [int]$resp.StatusCode
            Data   = $data
            Raw    = $resp.Content
        }
    }
    catch {
        $status  = 0
        $errBody = ""
        $resp    = $_.Exception.Response
        if ($resp) {
            $status = [int]$resp.StatusCode
            try {
                $reader  = New-Object System.IO.StreamReader($resp.GetResponseStream())
                $errBody = $reader.ReadToEnd()
            }
            catch { $errBody = "" }
        }
        if (-not $errBody -and $_.ErrorDetails.Message) {
            $errBody = $_.ErrorDetails.Message
        }
        if (-not $errBody) {
            $errBody = $_.Exception.Message
        }
        return [pscustomobject]@{
            Ok     = $false
            Status = $status
            Data   = $errBody
            Raw    = $errBody
        }
    }
}

function Write-Step {
    param([string]$Title)
    Write-Host ""
    Write-Host ("=" * 70) -ForegroundColor DarkGray
    Write-Host $Title -ForegroundColor Cyan
    Write-Host ("=" * 70) -ForegroundColor DarkGray
}

function Write-Ok {
    param([string]$Message)
    Write-Host "  [OK] $Message" -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host "  [!] $Message" -ForegroundColor Yellow
}

function Write-Fail {
    param([string]$Message, [int]$Status = 0)
    if ($Status -gt 0) {
        Write-Host "  [FALLO] (HTTP $Status) $Message" -ForegroundColor Red
    }
    else {
        Write-Host "  [FALLO] $Message" -ForegroundColor Red
    }
}

# =============================================
# FLUJO PRINCIPAL
# =============================================

Write-Host ""
Write-Host "Prueba integral de la API Heno Motita" -ForegroundColor Magenta
Write-Host "  Base: $BaseUrl" -ForegroundColor Gray

# ----- 1. SALUD ------------------------------------------------------------
Write-Step "1. Salud"

$health = Invoke-Api -Method "Get" -Path "/health"
if ($health.Ok) {
    Write-Ok "GET /health -> status=$($health.Data.status)"
}
else {
    Write-Fail "GET /health" -Status $health.Status
    Write-Warn "Verifica que la API este corriendo (go run ./cmd/api) y que $BaseUrl sea correcta."
    exit 1
}

$healthDb = Invoke-Api -Method "Get" -Path "/health/database"
if ($healthDb.Ok) {
    Write-Ok "GET /health/database -> $($healthDb.Data.message)"
}
else {
    Write-Fail "GET /health/database" -Status $healthDb.Status
    Write-Warn "Revisa MONGODB_URI en el archivo .env"
    exit 1
}

# ----- 2. LOGIN ------------------------------------------------------------
Write-Step "2. Login del super administrador"

$loginBody = @{ email = $AdminEmail; password = $AdminPassword } | ConvertTo-Json
$login = Invoke-Api -Method "Post" -Path "/api/v1/auth/login" -Body $loginBody

if (-not $login.Ok) {
    Write-Fail "POST /api/v1/auth/login -> $($login.Data)" -Status $login.Status
    Write-Warn "Revisa AdminEmail/AdminPassword o el seed del super administrador."
    exit 1
}

$token = $login.Data.accessToken
$authHeaders = @{ Authorization = "Bearer $token" }
Write-Ok "Login correcto como $($login.Data.user.name) (rol $($login.Data.user.role))"

# ----- 3. ME ---------------------------------------------------------------
Write-Step "3. Verificar sesion"

$me = Invoke-Api -Method "Get" -Path "/api/v1/auth/me" -Headers $authHeaders
if ($me.Ok) {
    Write-Ok "GET /auth/me -> $($me.Data.user.email) [$($me.Data.user.status)]"
}
else {
    Write-Fail "GET /auth/me" -Status $me.Status
    exit 1
}

# ----- 3.5 PRE-CHECK DEL TESTEMAIL ----------------------------------------
$existingStudent = $null

if ($TestEmail -ne "") {
    Write-Step "3.5 Comprobar si el correo de prueba ya existe"

    $search = [uri]::EscapeDataString($TestEmail)
    $history = Invoke-Api -Method "Get" `
        -Path "/api/v1/students/history?limit=100&search=$search" `
        -Headers $authHeaders

    if ($history.Ok -and $history.Data.students) {
        foreach ($item in $history.Data.students) {
            if ($item.student.email -ieq $TestEmail) {
                $existingStudent = $item
                break
            }
        }
    }

    if ($existingStudent) {
        Write-Warn "El correo $TestEmail ya esta registrado"
        Write-Host "    -> alumno: $($existingStudent.student.name) [$($existingStudent.student.status)]"
        Write-Host "    -> se generara un codigo nuevo via new-activation-code en el paso 7"
    }
    else {
        Write-Ok "El correo $TestEmail no existe. Se registrara en el paso 6."
    }
}

# ----- 4. CREAR MANAGER ----------------------------------------------------
Write-Step "4. Crear un encargado"

$stamp = Get-Date -Format "yyyyMMddHHmmss"
$managerEmail = "prueba.$stamp@escuela.edu"

$managerBody = @{
    name        = "Encargado Prueba $stamp"
    email       = $managerEmail
    password    = "Contrasena123"
    phone       = "7711234567"
    institution = "Escuela de Prueba"
} | ConvertTo-Json

$manager = Invoke-Api -Method "Post" -Path "/api/v1/managers" -Headers $authHeaders -Body $managerBody

if (-not $manager.Ok) {
    Write-Fail "POST /managers -> $($manager.Data)" -Status $manager.Status
    exit 1
}

$managerId = $manager.Data.manager.id
Write-Ok "Encargado creado: $managerEmail (id=$managerId)"

# ----- 5. CREAR CREW -------------------------------------------------------
Write-Step "5. Crear una cuadrilla"

$nowUtc  = (Get-Date).ToUniversalTime()
$startAt = $nowUtc.AddDays(-1).ToString("yyyy-MM-ddTHH:mm:ssZ")
$endAt   = $nowUtc.AddDays(60).ToString("yyyy-MM-ddTHH:mm:ssZ")

$crewBody = @{
    name         = "Cuadrilla Prueba $stamp"
    description  = "Prueba automatica de envio de correo"
    zone         = "Tula"
    institution  = "CBTIS 123"
    managerId    = $managerId
    startAt      = $startAt
    endAt        = $endAt
    studentLimit = 30
} | ConvertTo-Json

$crew = Invoke-Api -Method "Post" -Path "/api/v1/crews" -Headers $authHeaders -Body $crewBody

if (-not $crew.Ok) {
    Write-Fail "POST /crews -> $($crew.Data)" -Status $crew.Status
    exit 1
}

$crewId = $crew.Data.crew.id
Write-Ok "Cuadrilla creada: $($crew.Data.crew.name) (id=$crewId)"

# ----- 6. BATCH DE ALUMNOS NUEVOS -----------------------------------------
Write-Step "6. Registrar alumnos nuevos (batch)"

# El alumno de prueba se registra solo si NO existe en el sistema.
$registerTestEmail = ($TestEmail -ne "" -and -not $existingStudent)

$studentsList = @(
    @{
        name       = "Alumno Prueba $stamp"
        email      = "alumno.$stamp@gmail.com"
        enrollment = "T$stamp-1"
    }
)

if ($registerTestEmail) {
    $studentsList += @{
        name       = "Destino de prueba"
        email      = $TestEmail
        enrollment = "T$stamp-2"
    }
}

$studentsBody = @{ students = $studentsList } | ConvertTo-Json -Depth 5

$batch = Invoke-Api -Method "Post" -Path "/api/v1/crews/$crewId/students/batch" `
    -Headers $authHeaders -Body $studentsBody

if (-not $batch.Ok) {
    Write-Fail "POST /crews/:id/students/batch -> $($batch.Data)" -Status $batch.Status
    exit 1
}

Write-Ok "Alumnos registrados: $($batch.Data.registered)"

Write-Host ""
Write-Host "  Codigos de activacion generados (batch):" -ForegroundColor Yellow
foreach ($cred in $batch.Data.credentials) {
    Write-Host ("    {0,-40} {1,-16} expira: {2}" -f $cred.email, $cred.activationCode, $cred.expiresAt) -ForegroundColor Green
}

# ----- 7. ENVIAR CODIGO AL CORREO DE PRUEBA --------------------------------
$sentEmail = $null

if ($registerTestEmail) {
    $sentEmail = $TestEmail
}
elseif ($existingStudent) {
    Write-Step "7. Regenerar codigo para el correo ya registrado"

    $existingId = $existingStudent.student.id
    $newCode = Invoke-Api -Method "Post" `
        -Path "/api/v1/students/$existingId/new-activation-code" `
        -Headers $authHeaders

    if ($newCode.Ok) {
        $cred = $newCode.Data.credential
        Write-Ok "Nuevo codigo generado para $($cred.email): $($cred.activationCode)"
        Write-Host ("    expira: {0}" -f $cred.expiresAt) -ForegroundColor Gray
        $sentEmail = $cred.email
    }
    else {
        Write-Fail "POST /students/:id/new-activation-code -> $($newCode.Data)" -Status $newCode.Status
    }
}

if ($sentEmail) {
    Write-Host ""
    Write-Host "  >>> Revisa la bandeja de entrada (o Spam) de: $sentEmail" -ForegroundColor Magenta
    Write-Host "  >>> Debe llegar el correo 'Heno Motita - Tu codigo de activacion' con el codigo." -ForegroundColor Magenta
}
else {
    Write-Warn "No se definio TestEmail. Los codigos de la lista anterior se usan con su correo."
}

# ----- 8. ACTIVAR CUENTA ---------------------------------------------------
if (-not $SkipActivation) {
    Write-Step "8. Activar cuenta del primer alumno registrado en batch"

    $firstCred = $batch.Data.credentials[0]
    $activateBody = @{
        email          = $firstCred.email
        activationCode = $firstCred.activationCode
        password       = "NuevaPass123"
    } | ConvertTo-Json

    $activate = Invoke-Api -Method "Post" -Path "/api/v1/auth/activate" -Body $activateBody

    if ($activate.Ok) {
        Write-Ok $activate.Data.message
        Write-Ok "Login del alumno: $($firstCred.email) / NuevaPass123"
    }
    else {
        Write-Fail "POST /auth/activate -> $($activate.Data)" -Status $activate.Status
    }
}

# ----- 9. RESUMEN ----------------------------------------------------------
Write-Step "Resumen"

Write-Host "  Token (Bearer): $token" -ForegroundColor Gray
Write-Host "  Manager id:     $managerId" -ForegroundColor Gray
Write-Host "  Crew id:        $crewId" -ForegroundColor Gray

Write-Host ""
Write-Host "  COMPLETADO. Si los pasos 6 y 7 terminaron sin errores y SENDGRID_API_KEY esta"
Write-Host "  configurado, los codigos fueron enviados por correo." -ForegroundColor Green
