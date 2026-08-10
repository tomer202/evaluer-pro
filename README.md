# Evaluer

High-performance **REST API application** built in **Go** to calculate student grades, generate student recommendation queues, and provide course status breakdown analytics for the Hive LMS platform.

---

## ⚙️ Environment Variables

Configure the required environment variables in `.env` (or shell environment):

```bash
HIVE_BASE_URL="https://20.13.2.62"
HIVE_USERNAME="your_username"
HIVE_PASSWORD="your_password"
DATABASE_URL="postgresql+psycopg2://root:password@20.13.2.62:5432/core"
POSTGRES_HOST="20.13.2.62"
POSTGRES_PORT="5432"
POSTGRES_DB="core"
POSTGRES_USER="root"
POSTGRES_PASSWORD="password"
CONFIG_PATH=".local/config.yaml"       # Path to YAML course configuration
IGNORED_USERNAMES="test_user,demo"      # Comma-separated list of usernames to ignore
LOG_LEVEL="INFO"                        # Logging level (DEBUG, INFO, WARNING, ERROR)
```

---

## ⚡ Architecture & Performance Features

- **Real-Time Database Queries**: Every API request dynamically queries the PostgreSQL database in real-time to fetch up-to-date assignments, responses, and student grades.
- **SQL Target Student Filtering**: Filters student data at the SQL level (`WHERE user_id IN (...)`), reducing data payloads by 53x for student-specific queries.
- **High-Concurrency Parallel Prefetching**: Uses Go goroutines (`sync.WaitGroup`) to execute database entity queries concurrently over connection pool workers.
- **Sub-25ms Scalability**: Benchmarked for **4,000 exercises, 12 subjects, and 53 students** with $<25\text{ ms}$ response times.

---

## 📡 API Endpoints Reference & Examples

Interactive OpenAPI Swagger UI documentation is available at `http://localhost:8000/docs` (JSON spec at `http://localhost:8000/openapi.json`).

---

### 🔧 Configuration Endpoints

#### 1. `GET /api/v1/config`
Retrieves the active course configuration JSON.

- **Query Parameters**: *None*
- **Request Payload**: *None*
- **Response Example**:
```json
{
  "subjects": {
    "Mathematics": {
      "name": "Mathematics",
      "weight": 0.6,
      "modules": {
        "AlgebraBasics": {
          "name": "AlgebraBasics",
          "weight": 1.0,
          "exercises": {
            "QuadraticEq": {
              "name": "QuadraticEq",
              "score": 100.0
            }
          }
        }
      }
    }
  }
}
```

#### 2. `POST /api/v1/config/generate`
Generates a course configuration skeleton from Hive DB and optionally saves to disk.

- **Query Parameters**: `save` (boolean, default: `true`), `path` (string, optional)
- **Request Payload**: *None*
- **Response Example**:
```json
{
  "message": "Config successfully generated and saved to disk",
  "config_path": ".local/config.yaml",
  "config": {
    "subjects": {
      "Mathematics": {
        "name": "Mathematics",
        "weight": 1.0,
        "modules": {}
      }
    }
  }
}
```

#### 3. `POST /api/v1/config/generate/preview`
Generates a course configuration preview in-memory without modifying active settings or files.

- **Request Payload**: *None*
- **Response Example**: Returns generated `CourseConfig` JSON.

#### 4. `PUT /api/v1/config`
Full replacement of the course configuration YAML file.

- **Query Parameters**: `path` (string, optional)
- **Response Example**: Returns updated `CourseConfig` JSON.

#### 5. `PATCH /api/v1/config`
Partial update of the course configuration (e.g. updating subject/module weights or exercise scores).

- **Query Parameters**: `path` (string, optional)
- **Response Example**: Returns updated `CourseConfig` JSON.

---

### 📊 Grade Calculation Endpoints

#### 6. `POST /api/v1/calc/exercise`
Calculates grades for a specific exercise across target students.

- **Request Payload Example**:
```json
{
  "subject_name": "Mathematics",
  "module_name": "AlgebraBasics",
  "exercise_name": "QuadraticEq",
  "students": ["student1"]
}
```
- **Response Example**:
```json
{
  "subject_name": "Mathematics",
  "module_name": "AlgebraBasics",
  "exercise_name": "QuadraticEq",
  "grades": [
    {
      "student_id": 7,
      "username": "student1",
      "first_name": "John",
      "last_name": "Doe",
      "raw_grade": 100.0,
      "score": 100.0,
      "review": "",
      "status": "Done",
      "assignment_url": "https://20.13.2.62/course/13/29/89#7",
      "redo_count": 0,
      "is_checked": true
    }
  ]
}
```

#### 7. `POST /api/v1/calc/module`
Calculates grades for a specific module across target students.

- **Request Payload Example**:
```json
{
  "subject_name": "Mathematics",
  "module_name": "AlgebraBasics",
  "students": ["student1"]
}
```

#### 8. `POST /api/v1/calc/subject`
Calculates grades for a specific subject across target students.

- **Request Payload Example**:
```json
{
  "subject_name": "Mathematics",
  "students": ["student1"]
}
```

#### 9. `POST /api/v1/calc/course`
Calculates overall course grades across all target students.

- **Request Payload Example**:
```json
{
  "students": ["student1"]
}
```

#### 10. `POST /api/v1/calc/course/status`
Returns deep-dive status summary breakdown (`DONE`, `REDO`, `WORK_IN_PROGRESS`, `SUBMITTED`, `NEW`, `REDO_UNCHECKED`, `WIP_UNCHECKED`) across course, subjects, modules, and exercises. Unopened exercises are excluded.

- **Request Payload Example**:
```json
{
  "students": []
}
```

#### 11. `POST /api/v1/calc/student`
Aggregative endpoint returning a student's full course calculation hierarchically organized (**Subject ➔ Module ➔ Exercise**).

- **Request Payload Example**:
```json
{
  "username": "student1"
}
```

#### 12. `POST /api/v1/calc/student/recommendations`
Returns a Student Recommendation Queue suggesting prioritized exercises to raise subject proficiency up to `target_score`.

- **Request Payload Example**:
```json
{
  "username": "student1",
  "target_score": 70.0
}
```

---

## 🧪 Testing & Execution

- **Run Server**:
  ```powershell
  & "C:\Program Files\Go\bin\go.exe" run ./cmd/server
  ```

- **Run Full Test Suite**:
  ```powershell
  & "C:\Program Files\Go\bin\go.exe" test -v -count=1 ./...
  ```