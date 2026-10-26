# Harness file (protected). Atlas versioned migrations for apps/api. See docs/harness/rules.md#db-*.
# Dev database: throwaway Docker container Atlas starts itself (needs Docker).

variable "url" {
  type    = string
  default = getenv("DATABASE_URL")
}

lint {
  destructive {
    error = true # rules.md#db-no-destructive-migration
  }
  incompatible {
    error = true # rules.md#db-backward-compatible-migration
  }
}

env "local" {
  url = var.url
  dev = "docker://postgres/17/dev"
  migration {
    dir = "file://db/migrations"
  }
}

env "ci" {
  url = var.url
  dev = "docker://postgres/17/dev"
  migration {
    dir = "file://db/migrations"
  }
}
