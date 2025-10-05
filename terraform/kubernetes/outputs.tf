output "namespace" {
  value = kubernetes_namespace_v1.stockbot
}

output "external_secret" {
  value = kubernetes_manifest.external_secret.manifest
}

output "persistent_volume_claim" {
  value = kubernetes_persistent_volume_claim_v1.duckdb
}
