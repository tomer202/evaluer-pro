# Evaluer

Modular student evaluation platform supporting both **CLI** and **REST API** modes to interact with the Hive platform and calculate student grades.

### Environment Variables
Configure the required environment variables in `.env` (or shell environment):
```bash
HIVE_BASE_URL="https://20.13.2.62"
HIVE_USERNAME="your_username"
HIVE_PASSWORD="your_password"
DATABASE_URL="postgresql+psycopg2://root:password@20.13.2.62:5432/core"
CONFIG_PATH="config.yaml"          # Optional path override
CACHE_TTL_SECONDS=86400            # Cache TTL in seconds
CACHE_ENABLED=true                 # Global cache toggle (default: true)
LOG_LEVEL="INFO"                   # Logging level (DEBUG, INFO, WARNING, ERROR)
IGNORED_USERNAMES="test_user,demo" # Comma-separated list of usernames to ignore in calculations
```

---

## 🖥️ Usage Modes

API mode application returning structured JSON outputs.


## 📡 API Endpoints Reference & Examples

Interactive OpenAPI Swagger documentation is available at `http://localhost:8000/docs`.

---

### Configuration Endpoints

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
Generates a course configuration skeleton from Hive and optionally saves to disk.

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
- **Response Example**:
```json
{
  "subjects": {
    "Mathematics": {
      "name": "Mathematics",
      "weight": 1.0,
      "modules": {}
    }
  }
}
```

#### 4. `PUT /api/v1/config`
Full replacement of the course configuration YAML file.

- **Query Parameters**: `path` (string, optional)
- **Request Payload Example**:
```json
{
  "subjects": {
    "Mathematics": {
      "name": "Mathematics",
      "weight": 0.5,
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
- **Response Example**: Returns the updated `CourseConfig` JSON.

#### 5. `PATCH /api/v1/config`
Partial update of the course configuration (e.g. updating specific subject/module weights or exercise scores).

- **Query Parameters**: `path` (string, optional)
- **Request Payload Example**:
```json
{
  "subjects": {
    "Mathematics": {
      "weight": 0.8
    }
  }
}
```
- **Response Example**: Returns the updated `CourseConfig` JSON.

---

### Grade Calculation Endpoints

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
- **Response Example**:
```json
{
  "subject_name": "Mathematics",
  "module_name": "AlgebraBasics",
  "students": [
    {
      "student_id": 7,
      "username": "student1",
      "first_name": "John",
      "last_name": "Doe",
      "module_grade": 100.0,
      "exercises": [
        {
          "exercise_name": "QuadraticEq",
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
  ]
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
- **Response Example**:
```json
{
  "subject_name": "Mathematics",
  "students": [
    {
      "student_id": 7,
      "username": "student1",
      "first_name": "John",
      "last_name": "Doe",
      "subject_grade": 100.0,
      "modules": [
        {
          "module_name": "AlgebraBasics",
          "module_grade": 100.0
        }
      ]
    }
  ]
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
- **Response Example**:
```json
{
  "students": [
    {
      "student_id": 7,
      "username": "student1",
      "first_name": "John",
      "last_name": "Doe",
      "total_grade": 100.0,
      "subjects": [
        {
          "subject_name": "Mathematics",
          "subject_grade": 100.0
        }
      ]
    }
  ]
}
```

#### 10. `POST /api/v1/calc/course/status`
Returns deep-dive status summary breakdown (`DONE`, `REDO`, `WORK_IN_PROGRESS`, `SUBMITTED`, `NEW`, `REDO_UNCHECKED`, `WIP_UNCHECKED`) across course, subjects, modules, and exercises.

- **Request Payload Example**:
```json
{
  "students": []
}
```
- **Response Example**:
```json
{
  "summary": {
    "DONE": 45,
    "REDO": 12,
    "WORK_IN_PROGRESS": 8,
    "SUBMITTED": 5,
    "NEW": 30,
    "REDO_UNCHECKED": 2,
    "WIP_UNCHECKED": 1
  },
  "subjects": [
    {
      "subject_name": "Mathematics",
      "summary": {
        "DONE": 45,
        "REDO": 12,
        "WORK_IN_PROGRESS": 8,
        "SUBMITTED": 5,
        "NEW": 30,
        "REDO_UNCHECKED": 2,
        "WIP_UNCHECKED": 1
      },
      "modules": [
        {
          "module_name": "AlgebraBasics",
          "summary": {
            "DONE": 20,
            "REDO": 5,
            "WORK_IN_PROGRESS": 2,
            "SUBMITTED": 1,
            "NEW": 10,
            "REDO_UNCHECKED": 1,
            "WIP_UNCHECKED": 0
          },
          "exercises": [
            {
              "exercise_name": "QuadraticEq",
              "summary": {
                "DONE": 10,
                "REDO": 2,
                "WORK_IN_PROGRESS": 1,
                "SUBMITTED": 0,
                "NEW": 5,
                "REDO_UNCHECKED": 0,
                "WIP_UNCHECKED": 0
              }
            }
          ]
        }
      ]
    }
  ]
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
- **Response Example**:
```json
{
  "student_id": 7,
  "username": "student1",
  "first_name": "John",
  "last_name": "Doe",
  "total_grade": 100.0,
  "subjects": [
    {
      "subject_name": "Mathematics",
      "subject_grade": 100.0,
      "modules": [
        {
          "module_name": "AlgebraBasics",
          "module_grade": 100.0,
          "exercises": [
            {
              "exercise_name": "QuadraticEq",
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
      ]
    }
  ]
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
- **Response Example**:
```json
{
  "username": "student1",
  "target_score": 70.0,
  "subjects": [
    {
      "subject_name": "Mathematics",
      "current_grade": 50.0,
      "target_score": 70.0,
      "target_achieved": false,
      "projected_grade": 75.0,
      "recommended_queue": [
        {
          "subject_name": "Mathematics",
          "module_name": "AlgebraBasics",
          "exercise_name": "LinearEq",
          "current_status": "Redo",
          "current_score": 0.0,
          "max_score": 25.0,
          "assignment_url": "https://20.13.2.62/course/13/29/88#7"
        }
      ]
    }
  ]
}
```

---

## 📊 Student Recommendation Algorithm

The endpoint `POST /api/v1/calc/student/recommendations` builds sequential subject recommendation queues using the following rules:

### 1. Subject Proficiency Strategy (`target_score`)
- Accepts a `target_score` (default: `70.0`).
- Groups recommendations per subject and stops adding exercises as soon as `projected_grade >= target_score`.

### 2. Sequential Unreached Recommendations
- Exercises are recommended sequentially ($E_1 \rightarrow E_2 \rightarrow E_3$) after prerequisite exercises achieve $\ge 80\%$ grade percentage or `DONE` status.

### 3. Zero-Weight Filtering
- Exercises in zero-weight subjects (`weight <= 0.0`) or zero-weight modules (`weight <= 0.0`) are automatically excluded.

---

## 🛠️ Debugging

A [.vscode/launch.json](.vscode/launch.json) configuration is provided for debugging in VS Code:
- **Evaluer CLI: Config Generate**
- **Evaluer CLI: Calculate**
- **Evaluer API Mode**
- **Evaluer: Current File**