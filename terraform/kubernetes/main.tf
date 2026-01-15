locals {
  name = "stockbot"
}

resource "kubernetes_service_v1" "stockbot" {
  metadata {
    name      = "stockbot"
    namespace = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name
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


resource "kubernetes_persistent_volume_claim_v1" "stockbot" {
  metadata {
    name      = "stockbot"
    namespace = data.terraform_remote_state.kubernetes_cluster.outputs.discordbots.namespace.metadata.0.name
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
