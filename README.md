# search-aggregator
Works with the search-collector to synchronize multicloud resources in the redis cache


## Development

1. Start [RedisGraph](https://oss.redislabs.com/redisgraph/)
    ```
    docker run -p 6379:6379 -it --rm redislabs/redisgraph
    ```
2. Install dependencies (temp, will move to make)
    ```
    go get
    ```
3. Build and run (temp, will move to make)
    ```
    go build && ./search-aggregator
    ```

## Usage

1. GET http://localhost:8000/status

    **Response:**
    - Total number of clusters.

2. GET http://localhost:8000/cluster/[clustername]/status

    **Response:**
    - Timestamp of last update.
    - Total number of resources in the cluster.
    - The current hash for all resources in the cluster.

3. POST http://localhost:8000/cluster/[clustername]/sync

    - `hash` - value of the current hash, used to validate the collector is in sync with the current RedisGraph cache.
    - `clearAll` - delete all resources for the cluster before adding new resources.
    - `addResources` - List of resources to be added.
    - `updateResources` - List of resources to be updated.
    - `deleteResources` - List of resources to be deleted.
    **Sample body:**
    ```json
    {
      "hash": "hash-of-previous-state",
      "clearAll": false,
      "addResources": [
        {
          "kind": "kind1",
          "uid": "uid-of-new-resource",
          "hash": "hash-of-new-resource",
          "properties": {
            "name": "name1"
          }
        }
      ],
      "updateResources": [
        {
          "kind": "kind2",
          "uid": "uid-of-existing-resource",
          "hash": "hash-of-updated-resource",
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

    **Success Response:**
    - Hash - new hash
    - Total number of added, updated, and deleted resources.
    - Total number of resources.

    **Error Response:**
    - Message -

