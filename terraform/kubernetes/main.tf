locals {
  name = "stockbot"
}

resource "kubernetes_namespace_v1" "stockbot" {
  metadata {
    name = local.name
  }
}

resource "kubernetes_service_v1" "stockbot" {
  metadata {
    name      = "stockbot"
    namespace = kubernetes_namespace_v1.stockbot.id
  }
  spec {
    selector = {
      "app" = local.name
    }
    port {
      name        = "router"
      target_port = 8080
      port        = 80
    }
  }
}


resource "kubernetes_persistent_volume_claim_v1" "duckdb" {
  metadata {
    name      = "duckdb"
    namespace = kubernetes_namespace_v1.stockbot.id
  }
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        "storage" = "3Gi"
      }
    }
  }
}
