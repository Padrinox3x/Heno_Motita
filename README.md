# Heno_Motita
# Heno Motita API

API REST desarrollada en **Go** para la gestión de un programa de monitoreo ambiental basado en el método de evaluación **Hawksworth (escala 0-6)**, utilizado para medir la calidad del aire mediante la observación de líquenes en árboles.

El sistema permite administrar **cuadrillas (crews)** de trabajo, **encargados (managers)**, **alumnos (students)**, **árboles (trees)** registrados en campo, **observaciones** (evaluaciones Hawksworth) y **evidencia fotográfica** de cada observación, alojada en Cloudinary.

---

## Tabla de contenido

- [Arquitectura y stack](#arquitectura-y-stack)
- [Estructura del proyecto](#estructura-del-proyecto)
- [Modelo de datos](#modelo-de-datos)
- [Roles y control de acceso](#roles-y-control-de-acceso)
- [Configuración e instalación](#configuración-e-instalación)
- [Variables de entorno](#variables-de-entorno)
- [Ejecución](#ejecución)
- [Rutas de salud](#rutas-de-salud)
- [Autenticación](#autenticación)
- [Referencia de la API](#referencia-de-la-api)
  - [Auth](#módulo-auth)
  - [Managers (encargados)](#módulo-managers)
  - [Crews (cuadrillas)](#módulo-crews)
  - [Students (alumnos)](#módulo-students)
  - [Student History (historial de alumnos)](#módulo-student-history)
  - [Trees (árboles)](#módulo-trees)
  - [Observations (observaciones Hawksworth)](#módulo-observations)
  - [Observation Images (evidencia fotográfica)](#módulo-observation-images)
- [Convenciones de paginación y filtrado](#convenciones-de-paginación-y-filtrado)
- [Manejo de errores](#manejo-de-errores)

---

## Arquitectura y stack

| Componente         | Tecnología                                         |
|---------------------|-----------------------------------------------------|
| Lenguaje            | Go 1.26                                              |
| Framework HTTP      | [Gin](https://github.com/gin-gonic/gin)              |
| Base de datos       | MongoDB Atlas (driver oficial `mongo-driver/v2`)     |
| Autenticación       | JSON Web Tokens (`golang-jwt/jwt/v5`)                |
| Hash de contraseñas | `golang.org/x/crypto` (bcrypt)                       |
| Almacenamiento de imágenes | Cloudinary (`cloudinary-go/v2`)               |
| Variables de entorno| `joho/godotenv`                                      |

El proyecto sigue una arquitectura por capas dentro de `internal/`:

- **`config`** — Carga y valida variables de entorno (app y Cloudinary).
- **`database`** — Conexión a MongoDB, creación de índices y seed del superadministrador inicial.
- **`models`** — Entidades persistidas en MongoDB (`bson`).
- **`dto`** — Objetos de entrada/salida de la API (`json`), separados de los modelos para no exponer datos sensibles (p. ej. `passwordHash`).
- **`handlers`** — Controladores HTTP (uno por módulo).
- **`routes`** — Registro de rutas de Gin y aplicación de middlewares por endpoint.
- **`middleware`** — Autenticación JWT, autorización por rol y autorización por pertenencia a recursos (cuadrilla, árbol, observación).
- **`security`** — JWT, hash de contraseñas y generación de códigos de activación.
- **`services`** — Integración con Cloudinary.

---

## Estructura del proyecto

```
heno-motita-api/
├── cmd/
│   └── api/
│       └── main.go                 # Punto de entrada: carga config, conecta Mongo, registra rutas
├── internal/
│   ├── config/
│   │   ├── config.go                # Variables generales (Mongo, JWT, puerto, superadmin)
│   │   └── cloudinary.go            # Variables de Cloudinary
│   ├── database/
│   │   ├── mongodb.go               # Conexión a MongoDB Atlas
│   │   ├── indexes.go               # Índices de las colecciones
│   │   └── seed.go                  # Creación del SUPER_ADMIN inicial
│   ├── models/                      # Entidades: User, Crew, CrewMembership, Tree, Observation, ObservationImage
│   ├── dto/                         # Request/Response de cada módulo
│   ├── handlers/                    # Lógica de cada endpoint
│   ├── middleware/                  # Auth, roles, y acceso a crew/tree/observation
│   ├── routes/                      # Definición de rutas por módulo
│   └── security/                    # JWT, bcrypt, códigos de activación
├── .env.example
├── go.mod
└── go.sum
```

---

## Modelo de datos

### `User` (colección `users`)

Representa a **cualquier** usuario del sistema: superadministrador, encargado de cuadrilla o alumno. El rol determina qué campos aplican.

| Campo          | Tipo         | Notas |
|----------------|--------------|-------|
| `id`           | ObjectID     | |
| `name`         | string       | |
| `email`        | string       | Único |
| `enrollment`   | string       | Matrícula, solo aplica a `STUDENT` |
| `phone`        | string       | Uso principal en `CREW_MANAGER` |
| `institution`  | string       | Uso principal en `CREW_MANAGER` |
| `passwordHash` | string       | Nunca se expone en las respuestas (`json:"-"`) |
| `role`         | `SUPER_ADMIN` \| `CREW_MANAGER` \| `STUDENT` | |
| `status`       | `ACTIVE` \| `INACTIVE` \| `BLOCKED` | |
| `createdAt` / `updatedAt` | time.Time | |

### `Crew` (colección `crews`)

Cuadrilla de trabajo asignada a un encargado, con una zona, institución, periodo de vigencia (`startAt`/`endAt`) y cupo de alumnos.

Estados (`CrewStatus`): `PENDING`, `ACTIVE`, `FINISHED`, `CANCELLED`.

### `CrewMembership` (colección `memberships`)

Relaciona a un alumno con una cuadrilla en un periodo determinado. Un mismo alumno puede tener múltiples membresías históricas en distintas cuadrillas (por eso el historial vive separado del usuario). Guarda únicamente el **hash** del código de activación temporal, nunca el código en claro.

Estados (`MembershipStatus`): `PENDING`, `ACTIVE`, `INACTIVE`, `EXPIRED`, `REVOKED`.

### `Tree` (colección `trees`)

Árbol registrado dentro de una cuadrilla, con coordenadas geográficas, nombre común/científico y un código único (p. ej. `ARB-TULA-001`).

Estados (`TreeStatus`): `ACTIVE`, `INACTIVE`, `ARCHIVED`.

### `Observation` (colección `observations`)

Evaluación Hawksworth de un árbol. La escala evalúa tres tercios del árbol (`lowerThirdScore`, `middleThirdScore`, `upperThirdScore`), cada uno de **0 a 2 puntos**; el `totalScore` (0-6) **se calcula en el backend** y nunca se recibe del cliente.

Estados (`ObservationStatus`): `ACTIVE`, `ARCHIVED`.
Método (`AssessmentMethod`): `HAWKSWORTH_0_6`.

### `ObservationImage` (colección `observation_images`)

Evidencia fotográfica de una observación, almacenada en Cloudinary. Guarda metadatos como `assetId`, `publicId`, URLs, formato, dimensiones y tamaño en bytes.

---

## Roles y control de acceso

El sistema tiene tres roles jerárquicos:

| Rol | Descripción |
|-----|-------------|
| `SUPER_ADMIN` | Control total: gestiona encargados, cuadrillas, alumnos y puede consultar cualquier recurso. Se crea automáticamente al iniciar el servidor (seed) según las variables `SUPER_ADMIN_*`. |
| `CREW_MANAGER` | Encargado de una cuadrilla específica. Solo puede consultar/operar la cuadrilla que le fue asignada (`managerId`). |
| `STUDENT` | Alumno inscrito en una cuadrilla mediante una membresía activa. Registra árboles y observaciones, y solo puede editar lo que él mismo creó. |

La autorización combina dos tipos de middleware:

1. **`RequireAuth`** — Valida el JWT y adjunta el usuario autenticado (`currentUser`) al contexto, verificando además que siga existiendo y esté `ACTIVE` en MongoDB.
2. **`RequireRoles(...)`** — Restringe el endpoint a una lista de roles permitidos.
3. **Middlewares de pertenencia** (`RequireCrewAccess`, `RequireCrewTreeAccess`, `RequireTreeAccess`, `RequireTreeWriteAccess`, `RequireObservationAccess`, `RequireObservationWriteAccess`) — Verifican, más allá del rol, que el usuario tenga relación directa con el recurso solicitado:
   - Un `CREW_MANAGER` solo accede a la cuadrilla donde es `managerId`.
   - Un `STUDENT` solo accede a cuadrillas donde tiene una membresía **activa y vigente**, y solo puede **editar** árboles/observaciones/imágenes que él mismo creó (los middlewares de escritura, sufijo `WriteAccess`, aplican esta restricción adicional).

---

## Configuración e instalación

### Requisitos

- Go 1.26 o superior
- Una base de datos MongoDB Atlas (o instancia compatible)
- Una cuenta de Cloudinary (para el módulo de imágenes)

### Instalación

```bash
git clone https://github.com/Padrinox3x/Heno_Motita.git
cd heno-motita-api
go mod download
cp .env.example .env
# Edita .env con tus credenciales
```

---

## Variables de entorno

Basado en `.env.example`, más las variables adicionales que requiere `config.Load()` y `LoadCloudinarySettings()`:

| Variable | Obligatoria | Valor por defecto | Descripción |
|----------|:---:|---|---|
| `APP_ENV` | No | `development` | En `production` activa `gin.ReleaseMode`. |
| `PORT` | No | `8080` | Puerto donde escucha el servidor. |
| `MONGODB_URI` | **Sí** | — | Cadena de conexión a MongoDB Atlas. |
| `MONGODB_DATABASE` | No | `heno_motita` | Nombre de la base de datos. |
| `FRONTEND_URL` | No | `http://localhost:5173` | URL del frontend (referencia/CORS). |
| `SUPER_ADMIN_NAME` | **Sí** | — | Nombre del superadministrador inicial (seed). |
| `SUPER_ADMIN_EMAIL` | **Sí** | — | Correo del superadministrador inicial. |
| `SUPER_ADMIN_PASSWORD` | **Sí** | — | Contraseña inicial (mínimo 8 caracteres). |
| `JWT_ACCESS_SECRET` | **Sí** | — | Secreto para firmar el JWT (mínimo 32 caracteres). |
| `JWT_ISSUER` | No | `heno-motita-api` | Emisor incluido en el token. |
| `JWT_ACCESS_DURATION` | No | `15m` | Duración del access token (formato Go: `15m`, `1h`, etc.). |
| `CLOUDINARY_CLOUD_NAME` | **Sí** | — | Nombre de la cuenta de Cloudinary. |
| `CLOUDINARY_API_KEY` | **Sí** | — | API key de Cloudinary. |
| `CLOUDINARY_API_SECRET` | **Sí** | — | API secret de Cloudinary. |
| `CLOUDINARY_FOLDER` | No | `heno-motita` | Carpeta donde se almacenan las imágenes. |
| `MAX_IMAGE_SIZE_MB` | No | `8` | Tamaño máximo permitido por imagen, en MB. |
| `SMTP_HOST` | No | — | Servidor SMTP (p. ej. `smtp.gmail.com`). Si queda vacío, los correos se simulan en consola. |
| `SMTP_PORT` | No | `587` | Puerto SMTP (STARTTLS). |
| `SMTP_USER` | No | — | Correo del remitente (cuenta Gmail). |
| `SMTP_PASSWORD` | No | — | Contraseña de aplicación (App Password) de Gmail. |
| `SMTP_FROM` | No | — | Dirección "De" del correo (normalmente igual a `SMTP_USER`). |
| `SMTP_FROM_NAME` | No | `Heno Motita` | Nombre visible del remitente. |

> **Nota:** `.env.example` incluye `CLOUDINARY_URL` como referencia general, pero el código (`internal/config/cloudinary.go`) lee específicamente `CLOUDINARY_CLOUD_NAME`, `CLOUDINARY_API_KEY` y `CLOUDINARY_API_SECRET` por separado. Asegúrate de definir esas tres variables individualmente en tu `.env`.

> **Correo (SMTP):** el código de activación de cuentas se envía por correo con `internal/services/email_service.go`. En producción (Render) no existe `.env`, por lo que las variables `SMTP_*` deben definirse en el panel **Environment** del servicio. Si `SMTP_HOST` está vacío, el correo no se envía y su contenido se imprime en la consola (modo desarrollo).

---

## Ejecución

```bash
go run ./cmd/api
```

Al iniciar, el servidor:

1. Carga y valida la configuración.
2. Conecta a MongoDB Atlas.
3. Crea los índices necesarios (`database.EnsureIndexes`).
4. Verifica/crea el superadministrador inicial (`database.SeedSuperAdmin`).
5. Configura JWT y Cloudinary.
6. Registra todas las rutas y arranca en `0.0.0.0:$PORT`.

---

## Rutas de salud

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/health` | Verifica que la API esté en ejecución. Devuelve `status`, `message` y `environment`. |
| `GET` | `/health/database` | Hace `ping` a MongoDB. Devuelve `503` si la conexión falla. |

---

## Autenticación

Todas las rutas (salvo `/health`, `/api/v1/auth/login` y `/api/v1/auth/activate`) requieren un header:

```
Authorization: Bearer <accessToken>
```

El token se obtiene en `POST /api/v1/auth/login` y expira según `JWT_ACCESS_DURATION`.

---

## Referencia de la API

Todas las rutas están bajo el prefijo `/api/v1`. Los campos marcados como **obligatorio** deben incluirse en el cuerpo (`body`) o en la ruta (`params`) según corresponda.

### Módulo Auth

| Método | Ruta | Rol requerido | Descripción |
|---|---|---|---|
| `POST` | `/auth/login` | Público | Inicia sesión y devuelve un access token. |
| `GET` | `/auth/me` | Cualquier usuario autenticado | Devuelve los datos del usuario en sesión. |

**`POST /auth/login`**

Body:
```json
{
  "email": "usuario@ejemplo.com",
  "password": "********"
}
```

Respuesta `200 OK`:
```json
{
  "message": "Inicio de sesión correcto",
  "accessToken": "eyJhbGciOi...",
  "tokenType": "Bearer",
  "expiresIn": 900,
  "expiresAt": "2026-08-03T12:15:00Z",
  "user": {
    "id": "665f1c...",
    "name": "Ana Pérez",
    "email": "usuario@ejemplo.com",
    "role": "CREW_MANAGER",
    "status": "ACTIVE",
    "createdAt": "...",
    "updatedAt": "..."
  }
}
```

Posibles errores: `400` (datos faltantes/incorrectos), `401` (credenciales inválidas), `403` (cuenta no activa).

---

### Módulo Managers

Gestión de encargados de cuadrilla. **Todas las rutas requieren rol `SUPER_ADMIN`.**

| Método | Ruta | Descripción |
|---|---|---|
| `POST` | `/managers` | Crea un encargado. |
| `GET` | `/managers` | Lista encargados (paginado, con filtros). |
| `GET` | `/managers/:id` | Consulta un encargado por ID. |
| `PUT` | `/managers/:id` | Actualiza los datos de un encargado. |
| `PATCH` | `/managers/:id/status` | Cambia el estado (`ACTIVE`/`INACTIVE`/`BLOCKED`). |

**`POST /managers`** — Body:
```json
{
  "name": "Ana Pérez",
  "email": "ana@escuela.edu",
  "password": "contraseñaSegura123",
  "phone": "7711234567",
  "institution": "Escuela Primaria Benito Juárez"
}
```
Validaciones: `name` (3-120), `email` válido (máx. 160), `password` (8-72), `phone` opcional (máx. 20), `institution` (2-160).

**`GET /managers`** — Query params opcionales: `page`, `limit`, `status`, `search` (ver [convenciones de paginación](#convenciones-de-paginación-y-filtrado)).

**`PUT /managers/:id`** — Mismo body que la creación, pero `password` es **opcional** (si se omite, se conserva la actual).

**`PATCH /managers/:id/status`** — Body:
```json
{ "status": "BLOCKED" }
```

---

### Módulo Crews

Gestión de cuadrillas de trabajo.

| Método | Ruta | Rol requerido | Descripción |
|---|---|---|---|
| `POST` | `/crews` | `SUPER_ADMIN` | Crea una cuadrilla. |
| `GET` | `/crews` | `SUPER_ADMIN` | Lista todas las cuadrillas (paginado). |
| `GET` | `/crews/:id` | `SUPER_ADMIN` (cualquiera) / `CREW_MANAGER` (solo la propia) | Consulta una cuadrilla. |
| `PUT` | `/crews/:id` | `SUPER_ADMIN` | Actualiza una cuadrilla. |
| `PATCH` | `/crews/:id/status` | `SUPER_ADMIN` | Cambia el estado de la cuadrilla. |

**`POST /crews`** — Body:
```json
{
  "name": "Cuadrilla Tula Centro",
  "description": "Monitoreo en parque central",
  "zone": "Tula de Allende",
  "institution": "CBTIS 123",
  "managerId": "665f1c2b8a1e2f0012a3b456",
  "startAt": "2026-08-10T06:00:00Z",
  "endAt": "2026-12-15T06:00:00Z",
  "studentLimit": 30
}
```
Notas:
- `startAt`/`endAt` en formato **RFC3339**.
- `managerId` debe corresponder a un usuario `CREW_MANAGER` existente y activo.
- `studentLimit`: entero entre 1 y 100.

**`GET /crews`** — Query params: `page`, `limit`, `status` (`PENDING`/`ACTIVE`/`FINISHED`/`CANCELLED`), `managerId`, `search` (busca por nombre).

**`PUT /crews/:id`** — Mismo body que la creación (edición completa).

**`PATCH /crews/:id/status`** — Body:
```json
{ "status": "FINISHED" }
```

---

### Módulo Students

Gestión de alumnos, membresías y activación de cuentas.

| Método | Ruta | Rol requerido | Descripción |
|---|---|---|---|
| `POST` | `/auth/activate` | Público | Activa la cuenta de un alumno con su código temporal. |
| `POST` | `/crews/:id/students/batch` | `SUPER_ADMIN` | Registra alumnos en lote dentro de una cuadrilla. |
| `GET` | `/crews/:id/students` | `SUPER_ADMIN` (cualquiera) / `CREW_MANAGER` (solo la propia) | Lista los alumnos de una cuadrilla. |
| `GET` | `/students/:id` | `SUPER_ADMIN`, `CREW_MANAGER` | Consulta un alumno por ID. |
| `PUT` | `/students/:id` | `SUPER_ADMIN` | Actualiza los datos generales de un alumno. |
| `PATCH` | `/students/:id/status` | `SUPER_ADMIN` | Cambia el estado de la cuenta. |
| `POST` | `/students/:id/new-activation-code` | `SUPER_ADMIN` | Genera un nuevo código de activación. |

**`POST /crews/:id/students/batch`** — Body:
```json
{
  "students": [
    {
      "name": "Luis Hernández",
      "email": "luis.hernandez@alumno.edu",
      "enrollment": "2026001"
    }
  ]
}
```
- Máximo 100 alumnos por lote.
- Por cada alumno se crea el `User` (rol `STUDENT`), una `CrewMembership` y un **código de activación temporal**.

Respuesta `201 Created`:
```json
{
  "message": "...",
  "crewId": "665f...",
  "registered": 1,
  "credentials": [
    {
      "studentId": "665f...",
      "name": "Luis Hernández",
      "email": "luis.hernandez@alumno.edu",
      "enrollment": "2026001",
      "activationCode": "AB12CD34",
      "expiresAt": "2026-08-10T06:00:00Z"
    }
  ]
}
```
> El `activationCode` solo se devuelve **una vez**, en el momento de la creación; en la base de datos solo se guarda su hash.
>
> Además de devolverse en la respuesta, el código de activación se envía por correo SMTP al alumno. Esto ocurre en el registro masivo (`BatchCreate`), al generar un nuevo código (`new-activation-code`) y al reactivar a un alumno en otra cuadrilla.

**`POST /auth/activate`** (público) — Body:
```json
{
  "email": "luis.hernandez@alumno.edu",
  "activationCode": "AB12CD34",
  "password": "nuevaContraseñaSegura"
}
```
Establece la contraseña definitiva del alumno usando el código temporal enviado al registrarlo.

**`GET /crews/:id/students`** — Query params: `page`, `limit`, `status`, `search`.

**`POST /students/:id/new-activation-code`** — Genera y devuelve un nuevo código (útil si el alumno perdió el original o expiró).

---

### Módulo Student History

Historial de participación de los alumnos en distintas cuadrillas y reactivación. **Todas las rutas requieren rol `SUPER_ADMIN`.**

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/students/history` | Lista general de alumnos con su membresía más reciente. |
| `GET` | `/students/:id/memberships` | Historial completo de membresías de un alumno. |
| `POST` | `/crews/:id/students/:studentId/reactivate` | Incorpora a un alumno existente a una nueva cuadrilla. |

**`GET /students/history`** — Query params: `page`, `limit`, `search`, `status`.

**`GET /students/:id/memberships`** — Devuelve el detalle del alumno y **todas** sus membresías históricas (`total` incluido).

**`POST /crews/:id/students/:studentId/reactivate`** — Crea una nueva `CrewMembership` para un alumno que ya existe en el sistema, generando un nuevo código de activación temporal (`credential`), sin duplicar su cuenta de usuario.

---

### Módulo Trees

Gestión de árboles registrados dentro de una cuadrilla.

| Método | Ruta | Rol requerido | Descripción |
|---|---|---|---|
| `POST` | `/crews/:id/trees` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` | Registra un árbol en la cuadrilla. |
| `GET` | `/crews/:id/trees` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` | Lista los árboles de una cuadrilla. |
| `GET` | `/trees/:id` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` | Consulta un árbol por ID. |
| `PUT` | `/trees/:id` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` | Actualiza un árbol. |
| `PATCH` | `/trees/:id/status` | `SUPER_ADMIN`, `CREW_MANAGER` | Cambia el estado de un árbol. |

Acceso: `RequireCrewTreeAccess` / `RequireTreeAccess` / `RequireTreeWriteAccess` validan que el usuario pertenezca a la cuadrilla dueña del árbol (membresía activa para alumnos, `managerId` para encargados).

**`POST /crews/:id/trees`** — Body:
```json
{
  "code": "ARB-TULA-001",
  "commonName": "Fresno",
  "scientificName": "Fraxinus uhdei",
  "latitude": 20.0500,
  "longitude": -99.3400,
  "locationDescription": "Entrada principal del parque"
}
```

**`GET /crews/:id/trees`** — Query params: `page`, `limit`, `status` (`ACTIVE`/`INACTIVE`/`ARCHIVED`), `search` (por código o nombre).

**`PATCH /trees/:id/status`** — Body:
```json
{ "status": "ARCHIVED" }
```

---

### Módulo Observations

Registro de evaluaciones **Hawksworth (0-6)** sobre un árbol.

| Método | Ruta | Rol requerido | Descripción |
|---|---|---|---|
| `POST` | `/trees/:id/observations` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` | Registra una observación sobre un árbol. |
| `GET` | `/trees/:id/observations` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` | Lista las observaciones de un árbol. |
| `GET` | `/observations/:id` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` | Consulta una observación por ID. |
| `PUT` | `/observations/:id` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` (solo si es el autor) | Edita una observación. |
| `PATCH` | `/observations/:id/status` | `SUPER_ADMIN`, `CREW_MANAGER` | Archiva/reactiva una observación. |

**`POST /trees/:id/observations`** — Body:
```json
{
  "lowerThirdScore": 1,
  "middleThirdScore": 2,
  "upperThirdScore": 0,
  "notes": "Cobertura liquénica dispersa en el tercio medio",
  "observationDate": "2026-08-03T09:30:00Z",
  "latitude": 20.0501,
  "longitude": -99.3399
}
```
- Cada puntaje (`lowerThirdScore`, `middleThirdScore`, `upperThirdScore`) va de **0 a 2**.
- `totalScore` (0-6) se calcula automáticamente en el servidor.
- `latitude`/`longitude` son opcionales.

Respuesta (fragmento):
```json
{
  "id": "...",
  "treeId": "...",
  "crewId": "...",
  "hawksworth": {
    "lowerThirdScore": 1,
    "middleThirdScore": 2,
    "upperThirdScore": 0,
    "totalScore": 3,
    "maximumScore": 6,
    "minimumScore": 0,
    "maximumByThird": 2
  },
  "status": "ACTIVE"
}
```

**`GET /trees/:id/observations`** — Query params: `page`, `limit`, `status` (`ACTIVE`/`ARCHIVED`).

**`PUT /observations/:id`** — Un `STUDENT` únicamente puede editar observaciones que él mismo registró (`RequireObservationWriteAccess`).

**`PATCH /observations/:id/status`** — Solo `SUPER_ADMIN` o `CREW_MANAGER` pueden archivar. Body:
```json
{ "status": "ARCHIVED" }
```

---

### Módulo Observation Images

Evidencia fotográfica asociada a una observación, almacenada en Cloudinary.

| Método | Ruta | Rol requerido | Descripción |
|---|---|---|---|
| `POST` | `/observations/:id/images` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` (solo autor de la observación) | Sube una imagen. |
| `GET` | `/observations/:id/images` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` | Lista las imágenes de una observación. |
| `DELETE` | `/observations/:id/images/:imageId` | `SUPER_ADMIN`, `CREW_MANAGER`, `STUDENT` (solo autor de la observación) | Elimina una imagen. |

**`POST /observations/:id/images`** — `multipart/form-data`:

| Campo | Tipo | Obligatorio | Descripción |
|---|---|:---:|---|
| `image` | archivo | Sí | Formatos permitidos: `image/jpeg`, `image/png`, `image/webp`. |
| `description` | texto | No | Máximo 500 caracteres. |

Reglas:
- No se pueden subir imágenes a una observación con estado `ARCHIVED` (`409 Conflict`).
- El tamaño máximo por imagen se controla con `MAX_IMAGE_SIZE_MB` (por defecto 8 MB); excederlo devuelve error.
- Las imágenes se suben a Cloudinary dentro de la carpeta configurada en `CLOUDINARY_FOLDER`.

**`GET /observations/:id/images`** — Query params: `page`, `limit`.

Respuesta (elemento de la lista):
```json
{
  "id": "...",
  "observationId": "...",
  "treeId": "...",
  "crewId": "...",
  "uploadedById": "...",
  "assetId": "...",
  "publicId": "...",
  "url": "http://res.cloudinary.com/...",
  "secureUrl": "https://res.cloudinary.com/...",
  "originalFilename": "foto1.jpg",
  "format": "jpg",
  "mimeType": "image/jpeg",
  "width": 1200,
  "height": 900,
  "bytes": 458213,
  "description": "Vista del tercio superior",
  "createdAt": "..."
}
```

**`DELETE /observations/:id/images/:imageId`** — Elimina el registro y el recurso correspondiente en Cloudinary.

---

## Convenciones de paginación y filtrado

Los endpoints de listado (`GET` que devuelven colecciones) comparten el mismo patrón de query params:

| Parámetro | Descripción | Reglas |
|---|---|---|
| `page` | Número de página | Entero ≥ 1 (por defecto `1`) |
| `limit` | Elementos por página | Entero entre 1 y 100 (por defecto `10`) |
| `search` | Búsqueda por texto | Se aplica con regex sobre campos como nombre/código (según el módulo) |
| `status` | Filtro por estado | Debe ser uno de los valores válidos del enum correspondiente al módulo |

Toda respuesta paginada incluye un objeto `pagination`:
```json
{
  "pagination": {
    "page": 1,
    "limit": 10,
    "total": 42,
    "totalPages": 5
  }
}
```

---

## Manejo de errores

Todas las respuestas de error siguen el mismo formato:

```json
{
  "status": "error",
  "message": "Descripción legible del error",
  "details": "Información adicional (opcional, p. ej. errores de validación)"
}
```

Códigos HTTP utilizados con más frecuencia:

| Código | Significado |
|---|---|
| `400` | Body o query params inválidos (validaciones de `binding`, formato de IDs, etc.) |
| `401` | Token ausente, inválido o expirado |
| `403` | El usuario autenticado no tiene permisos para el recurso/acción |
| `404` | El recurso solicitado no existe |
| `409` | Conflicto de negocio (p. ej. subir imágenes a una observación archivada) |
| `500` | Error interno (fallas de conexión a MongoDB/Cloudinary, etc.) |

---

## Notas finales

- Los IDs de MongoDB (`ObjectID`) se exponen siempre como strings hexadecimales de 24 caracteres.
- Las fechas se manejan en formato **RFC3339** (UTC recomendado).
- Ningún endpoint devuelve `passwordHash` ni el código de activación en claro después de su creación inicial.
- El middleware `RequireAuth` valida el JWT **y** vuelve a consultar el usuario en MongoDB en cada petición, para reflejar de inmediato bloqueos o desactivaciones de cuenta.
