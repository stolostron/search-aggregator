# search-aggregator
Works with [search-collector](https://github.com/open-cluster-management/search-collector) to synchronize kubernetes resources into [RedisGraph](https://oss.redislabs.com/redisgraph/), for use by the search UI. Runs on the hub cluster and receives from the collectors running on each remote cluster.

  
## Development

1. Start [RedisGraph](https://oss.redislabs.com/redisgraph/)
    ```
    docker run -p 6379:6379 -it --rm redislabs/redisgraph
    ```
2. Install dependencies
    ```
    make deps
    ```
3. Build and run
    ```
    make build && ./output/search-aggregator
    ```

## API Usage

1. **(currently unused)** GET https://localhost:3010/aggregator/status

    **Response:**
    - Total number of clusters.

2. **(currently unused)** GET https://localhost:3010/aggregator/clusters/[clustername]/status

    **Response:**
    - Timestamp of last update.
    - Total number of resources in the cluster.
    - Total number of intra edges in the cluster.
    - MaxQueueTime - used to tell collectors how often to send updates.

3. POST https://localhost:3010/aggregator/clusters/[clustername]/sync

    - `clearAll` - delete all resources for the cluster before adding new resources.
    - `addResources` - List of resources to be added.
    - `updateResources` - List of resources to be updated.
    - `deleteResources` - List of resources to be deleted.

    **Sample body:**
    ```json
    {
      "clearAll": false,
      "addResources": [
        {
          "kind": "kind1",
          "uid": "uid-of-new-resource",
          "properties": {
            "name": "name1"
          }
        }
      ],
      "updateResources": [
        {
          "kind": "kind2",
          "uid": "uid-of-existing-resource",
          "properties": {
            "name": "name2"
          }
        }
      ],
      "deleteResources": [
        {
          "uid": "uid-of-resource-to-delete"
        }
      ]
    }
    ```

    **Sample Response:**
    ```json
    {
        "TotalAdded": 1,
        "TotalUpdated": 2,
        "TotalDeleted": 3,
        "TotalResources": 4,
        "UpdatedTimestamp": 12345678,
        "AddErrors": [
            {
                "ResourceUID": "111aaa",
                "Message": "Failed to insert because ..."
            }
        ],
        "UpdateErrors": [
            {
                "ResourceUID": "222bbb",
                "Message": "Failed to update because ..."
            }
        ],
        "DeleteErrors": [
            {
                "ResourceUID": "333ccc",
                "Message": "Failed to delete because ..."
            }
        ],
    }
    ```
