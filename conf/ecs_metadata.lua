local function presence(value)
  local type = type

  if "string" == type(value) then
    if value == "" then
      return nil
    end
  elseif "table" == type(value) then
    local next = next

    if next(value) == nil then
      return nil
    end
  end

  return value
end

local ECS_CLUSTER_NAME  = presence(os.getenv("ECS_CLUSTER_NAME"))
local ECS_SERVICE_NAME  = presence(os.getenv("ECS_SERVICE_NAME"))

local ECS_TASK_ID       = presence(os.getenv("ECS_TASK_ID"))
local ECS_TASK_FAMILY   = presence(os.getenv("ECS_TASK_FAMILY"))
local ECS_TASK_REVISION = presence(os.getenv("ECS_TASK_REVISION"))

function ecs_metadata(tag, timestamp, record)
  local presence       = presence
  local container_id   = presence(record["container_id"])
  local container_name = presence(record["container_name"])

  record["container_id"]   = nil
  record["container_name"] = nil

  local cluster   = ECS_CLUSTER_NAME
  local service   = ECS_SERVICE_NAME
  local task      = presence({ id = ECS_TASK_ID, family = ECS_TASK_FAMILY, revision = ECS_TASK_REVISION })
  local container = presence({ id = container_id, name = container_name })

  if cluster or service or task or container then
    record["aws"] = {
      ecs = {
        cluster   = cluster,
        service   = service,
        task      = task,
        container = container
      }
    }

    return 2, timestamp, record
  end

  return 0, timestamp, record
end
