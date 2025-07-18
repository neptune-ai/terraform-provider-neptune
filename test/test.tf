terraform {
  required_providers {
    neptune = {
      source = "neptune-ai/neptune"
    }
  }
}

provider "neptune" {
  neptune_token = "eyJhcGlfYWRkcmVzcyI6Imh0dHBzOi8vbWFuYWdlLXVzZXItaWFhYy5kZXYubmVwdHVuZS5haSIsImFwaV91cmwiOiJodHRwczovL21hbmFnZS11c2VyLWlhYWMuZGV2Lm5lcHR1bmUuYWkiLCJhcGlfa2V5IjoiMzE2NGI3NTctOGM2My00ZjYwLTk3OTMtYTdmOGM1YzZkYWYyIn0="
  workspace     = "team"
}

resource "neptune_project" "test3" {
  name        = "test"
  description = "asdasdddd"
  color       = "#ff0000"
}

resource "neptune_project" "test" {
  count = 10
  name  = "test-${count.index}"
  description = "description-${count.index}"

}

resource "neptune_project" "test123" {
  count = 1
  name  = "workspace-${count.index}"
  description = "workspace-${count.index}"
  visibility = "workspace"
  color = "#000000"
}


resource "neptune_project_email_assignment" "test_assignment" {
  count   = 10
  project = neptune_project.test[count.index].id
  email   = "user${count.index}@gmail.com"
  role    = "manager"
}




# data "neptune_project" "data" {
#   project_name = "data"
# }

output "test" {
  value = resource.neptune_project.test
}

# output "data" {
#   value = data.neptune_project.data
# }
