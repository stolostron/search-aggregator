# search-aggregator
The Search Aggregator is a component of the Open Cluster Management [search feature](https://github.com/open-cluster-management/search/blob/master/feature-spec/search.md#feature-summary). It runs on the hub cluster to index data from [search-collector](https://github.com/open-cluster-management/search-collector) on the managed clusters into [RedisGraph](https://oss.redislabs.com/redisgraph/).


## Local Development

1. Start [RedisGraph](https://oss.redislabs.com/redisgraph/)
    ```
    docker run -p 6379:6379 -it --rm redislabs/redisgraph
    ```
2. Generate self-signed certificate for development
   ```
   sh setup.sh
   ```
3. Install dependencies
    ```
    make deps
    ```
4. Build and run
    ```
    make run
    ```

### Environment Variables
Control the behavior of this service with these environment variables.

Name                | Required | Default Value | Description
----                | -------- | ------------- | -----------
EDGE_BUILD_RATE_MS  | no       | 15000         | How often inter-cluster edges are re-calculated
HTTP_TIMEOUT        | no       | 300000        | Timeout to process a single requests
REDISCOVER_RATE_MS  | no       | 300000        | How often we check for new crds
REDIS_HOST          | yes      | localhost     | RedisGraph host
REDIS_PORT          | yes      | 6379          | RedisGraph port
REDIS_WATCH_INTERVAL| no       | 15000         | Check connection to RedisGraph
REQUEST_LIMIT       | no       | 10            | Max number of concurrent requests


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
