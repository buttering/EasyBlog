---
title: Buildfarm 任务队列简要分析
date: 2024-08-15 18:51:29
toc: true
mathjax: true
tags:
- buildfarm
- 构建系统
- 消息队列
- redis
---


Buildfarm中，Server 与 Worker之间通过redis传递消息，进行协作
## 任务实体
Server向redis写入形如下面的json字符串，以发布任务。
```json
{
  "operationName": "shard/operations/b5f16f00-edbf-4ac4-b4b8-5aabca68def5",
  "actionDigest": {
    "hash": "0a6f00652eb48c2f250549b8e5ff6982f4526f7e8819257da14dbe33c8ff5b6b",
    "sizeBytes": "142"
  },
  "requestMetadata": {
  },
  "executionPolicy": {
  },
  "resultsCachePolicy": {
  },
  "stdoutStreamName": "shard/operations/b5f16f00-edbf-4ac4-b4b8-5aabca68def5/streams/stdout",
  "stderrStreamName": "shard/operations/b5f16f00-edbf-4ac4-b4b8-5aabca68def5/streams/stderr",
  "queuedTimestamp": "2024-07-12T07:58:35.342Z"
}
```
相关代码：
```java
public void prequeue(ExecuteEntry executeEntry, Operation operation) throws IOException {
    String invocationId = extractInvocationId(operation);
    String operationName = operation.getName();
    String operationJson = operationPrinter.print(operation);
    String executeEntryJson = JsonFormat.printer().print(executeEntry);
    Operation publishOperation = onPublish.apply(operation);
    int priority = executeEntry.getExecutionPolicy().getPriority();
    client.run(
        jedis -> {
          state.operations.insert(jedis, invocationId, operationName, operationJson);
          state.prequeue.push(jedis, executeEntryJson, priority);
          publishReset(jedis, publishOperation);
        });
  }
```
## 协作流程
以下是某次构建任务前后redis的日志：
```powershell
1720948474.655909 [0 172.26.0.4:34072] "BRPOPLPUSH" "{Arrival}:PreQueuedOperations" "{Arrival}:PreQueuedOperations_dequeue" "1"
1720948474.756578 [0 172.26.0.3:37702] "BRPOPLPUSH" "{06S}cpu" "{06S}cpu_dequeue" "1"
1720948474.857072 [0 172.26.0.7:36552] "BRPOPLPUSH" "{Arrival}:PreQueuedOperations" "{Arrival}:PreQueuedOperations_dequeue" "1"
1720948474.957548 [0 172.26.0.6:59486] "BRPOPLPUSH" "{06S}cpu" "{06S}cpu_dequeue" "1"
1720948475.149266 [0 172.26.0.6:35568] "PING"
1720948475.313805 [0 172.26.0.6:35568] "LRANGE" "{Arrival}:PreQueuedOperations_dequeue" "0" "9999"
1720948475.313912 [0 172.26.0.6:35568] "LRANGE" "{06S}cpu_dequeue" "0" "9999"
1720948475.427030 [0 172.26.0.7:36546] "LLEN" "{06S}cpu"
1720948475.427122 [0 172.26.0.7:36546] "HGETALL" "DispatchedOperations"
1720948475.551394 [0 172.26.0.4:40696] "LLEN" "{06S}cpu"
1720948475.551511 [0 172.26.0.4:40696] "HGETALL" "DispatchedOperations"
1720948475.561652 [0 172.26.0.4:40696] "HGETALL" "Workers_storage"
1720948475.580667 [0 172.26.0.4:40696] "GET" "ActionCache:37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9/224"
1720948475.582439 [0 172.26.0.4:40696] "LLEN" "{Arrival}:PreQueuedOperations"
1720948475.583862 [0 172.26.0.4:40696] "SETEX" "Operation:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "604800" "{\n  \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n  \"metadata\": {\n    \"@type\": \"type.googleapis.com/build.bazel.remote.execution.v2.ExecuteOperationMetadata\",\n    \"actionDigest\": {\n      \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n      \"sizeBytes\": \"224\"\n    },\n    \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n    \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\"\n  }\n}"
1720948475.584001 [0 172.26.0.4:40696] "SADD" "" "shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.584091 [0 172.26.0.4:40696] "LPUSH" "{Arrival}:PreQueuedOperations" "{\n  \"operationName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n  \"actionDigest\": {\n    \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n    \"sizeBytes\": \"224\"\n  },\n  \"requestMetadata\": {\n  },\n  \"executionPolicy\": {\n  },\n  \"resultsCachePolicy\": {\n  },\n  \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n  \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\",\n  \"queuedTimestamp\": \"2024-07-14T09:14:35.582Z\"\n}"
1720948475.584850 [0 172.26.0.4:34072] "PING"
1720948475.584918 [0 172.26.0.4:34072] "PUBLISH" "OperationChannel:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "{\n  \"effectiveAt\": \"2024-07-14T09:14:35.584Z\",\n  \"source\": \"buildfarm-server-172.26.0.4:8980-3f5c7e02-910b-40f1-a204-4d0249730d90-shard\",\n  \"reset\": {\n    \"expiresAt\": \"2024-07-14T09:14:45.584Z\",\n    \"operation\": {\n      \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n      \"metadata\": {\n        \"@type\": \"type.googleapis.com/build.bazel.remote.execution.v2.ExecuteOperationMetadata\",\n        \"actionDigest\": {\n          \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n          \"sizeBytes\": \"224\"\n        },\n        \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n        \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\"\n      }\n    }\n  }\n}"
1720948475.585110 [0 172.26.0.4:40490] "SUBSCRIBE" "OperationChannel:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.585194 [0 172.26.0.4:34072] "PING"
1720948475.585263 [0 172.26.0.4:34072] "PUBLISH" "OperationChannel:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "{\n  \"effectiveAt\": \"2024-07-14T09:14:35.584Z\",\n  \"source\": \"buildfarm-server-172.26.0.4:8980-3f5c7e02-910b-40f1-a204-4d0249730d90-shard\",\n  \"reset\": {\n    \"expiresAt\": \"2024-07-14T09:14:45.584Z\",\n    \"operation\": {\n      \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\"\n    }\n  }\n}"
1720948475.585383 [0 172.26.0.4:34072] "LREM" "{Arrival}:PreQueuedOperations_dequeue" "-1" "{\n  \"operationName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n  \"actionDigest\": {\n    \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n    \"sizeBytes\": \"224\"\n  },\n  \"requestMetadata\": {\n  },\n  \"executionPolicy\": {\n  },\n  \"resultsCachePolicy\": {\n  },\n  \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n  \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\",\n  \"queuedTimestamp\": \"2024-07-14T09:14:35.582Z\"\n}"
1720948475.585502 [0 172.26.0.4:34072] "DEL" "Processing:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.586219 [0 172.26.0.4:34072] "SETEX" "Operation:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "604800" "{\n  \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n  \"metadata\": {\n    \"@type\": \"type.googleapis.com/build.bazel.remote.execution.v2.ExecuteOperationMetadata\",\n    \"stage\": \"CACHE_CHECK\",\n    \"actionDigest\": {\n      \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n      \"sizeBytes\": \"224\"\n    }\n  }\n}"
1720948475.586340 [0 172.26.0.4:34072] "SADD" "" "shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.587181 [0 172.26.0.4:34072] "PING"
1720948475.587244 [0 172.26.0.4:34072] "PUBLISH" "OperationChannel:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "{\n  \"effectiveAt\": \"2024-07-14T09:14:35.586Z\",\n  \"source\": \"buildfarm-server-172.26.0.4:8980-3f5c7e02-910b-40f1-a204-4d0249730d90-shard\",\n  \"reset\": {\n    \"expiresAt\": \"2024-07-14T09:14:45.586Z\",\n    \"operation\": {\n      \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n      \"metadata\": {\n        \"@type\": \"type.googleapis.com/build.bazel.remote.execution.v2.ExecuteOperationMetadata\",\n        \"stage\": \"CACHE_CHECK\",\n        \"actionDigest\": {\n          \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n          \"sizeBytes\": \"224\"\n        }\n      }\n    }\n  }\n}"
1720948475.587773 [0 172.26.0.4:34072] "GET" "ActionCache:37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9/224"
1720948475.587785 [0 172.26.0.4:40696] "LLEN" "{06S}cpu"
1720948475.587870 [0 172.26.0.4:34072] "RPOPLPUSH" "{Arrival}:PreQueuedOperations" "{Arrival}:PreQueuedOperations_dequeue"
1720948475.588005 [0 172.26.0.4:34072] "BRPOPLPUSH" "{Arrival}:PreQueuedOperations" "{Arrival}:PreQueuedOperations_dequeue" "1"
1720948475.594798 [0 172.26.0.4:40696] "LLEN" "{06S}cpu"
1720948475.596971 [0 172.26.0.4:40696] "SETEX" "Operation:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "604800" "{\n  \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n  \"metadata\": {\n    \"@type\": \"type.googleapis.com/build.buildfarm.v1test.QueuedOperationMetadata\",\n    \"executeOperationMetadata\": {\n      \"stage\": \"QUEUED\",\n      \"actionDigest\": {\n        \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n        \"sizeBytes\": \"224\"\n      },\n      \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n      \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\"\n    },\n    \"queuedOperationDigest\": {\n      \"hash\": \"07053c77251fea1a4deae25b26c42860372d2710a026be941728691ec62b5424\",\n      \"sizeBytes\": \"834\"\n    },\n    \"requestMetadata\": {\n    }\n  }\n}"
1720948475.597150 [0 172.26.0.4:40696] "SADD" "" "shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.597220 [0 172.26.0.4:40696] "HDEL" "DispatchedOperations" "shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.597345 [0 172.26.0.4:40696] "LPUSH" "{06S}cpu" "{\n  \"executeEntry\": {\n    \"operationName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n    \"actionDigest\": {\n      \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n      \"sizeBytes\": \"224\"\n    },\n    \"requestMetadata\": {\n    },\n    \"executionPolicy\": {\n    },\n    \"resultsCachePolicy\": {\n    },\n    \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n    \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\",\n    \"queuedTimestamp\": \"2024-07-14T09:14:35.582Z\"\n  },\n  \"queuedOperationDigest\": {\n    \"hash\": \"07053c77251fea1a4deae25b26c42860372d2710a026be941728691ec62b5424\",\n    \"sizeBytes\": \"834\"\n  },\n  \"platform\": {\n    \"properties\": [{\n      \"name\": \"container-image\",\n      \"value\": \"reg.docker.alibaba-inc.com/apsara-citest/janus-union:go1.20\"\n    }]\n  }\n}"
1720948475.598084 [0 172.26.0.4:40696] "PING"
1720948475.598156 [0 172.26.0.4:40696] "PUBLISH" "OperationChannel:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "{\n  \"effectiveAt\": \"2024-07-14T09:14:35.597Z\",\n  \"source\": \"buildfarm-server-172.26.0.4:8980-3f5c7e02-910b-40f1-a204-4d0249730d90-shard\",\n  \"reset\": {\n    \"expiresAt\": \"2024-07-14T09:14:45.597Z\",\n    \"operation\": {\n      \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n      \"metadata\": {\n        \"@type\": \"type.googleapis.com/build.bazel.remote.execution.v2.ExecuteOperationMetadata\",\n        \"stage\": \"QUEUED\",\n        \"actionDigest\": {\n          \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n          \"sizeBytes\": \"224\"\n        },\n        \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n        \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\"\n      }\n    }\n  }\n}"
1720948475.599579 [0 172.26.0.3:37702] "PING"
1720948475.599731 [0 172.26.0.3:37702] "PUBLISH" "OperationChannel:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "{\n  \"effectiveAt\": \"2024-07-14T09:14:35.598Z\",\n  \"source\": \"buildfarm-worker-172.26.0.3:8981-fc464f78-6432-428b-a7f0-a43d6aa038ee\",\n  \"reset\": {\n    \"expiresAt\": \"2024-07-14T09:14:45.598Z\",\n    \"operation\": {\n      \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\"\n    }\n  }\n}"
1720948475.600506 [0 172.26.0.3:37702] "HSETNX" "DispatchedOperations" "shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "{\n  \"queueEntry\": {\n    \"executeEntry\": {\n      \"operationName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n      \"actionDigest\": {\n        \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n        \"sizeBytes\": \"224\"\n      },\n      \"requestMetadata\": {\n      },\n      \"executionPolicy\": {\n      },\n      \"resultsCachePolicy\": {\n      },\n      \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n      \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\",\n      \"queuedTimestamp\": \"2024-07-14T09:14:35.582Z\"\n    },\n    \"queuedOperationDigest\": {\n      \"hash\": \"07053c77251fea1a4deae25b26c42860372d2710a026be941728691ec62b5424\",\n      \"sizeBytes\": \"834\"\n    },\n    \"platform\": {\n      \"properties\": [{\n        \"name\": \"container-image\",\n        \"value\": \"reg.docker.alibaba-inc.com/apsara-citest/janus-union:go1.20\"\n      }]\n    }\n  },\n  \"requeueAt\": \"1720948485599\"\n}"
1720948475.600724 [0 172.26.0.3:37702] "LREM" "{06S}cpu_dequeue" "-1" "{\n  \"executeEntry\": {\n    \"operationName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n    \"actionDigest\": {\n      \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n      \"sizeBytes\": \"224\"\n    },\n    \"requestMetadata\": {\n    },\n    \"executionPolicy\": {\n    },\n    \"resultsCachePolicy\": {\n    },\n    \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n    \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\",\n    \"queuedTimestamp\": \"2024-07-14T09:14:35.582Z\"\n  },\n  \"queuedOperationDigest\": {\n    \"hash\": \"07053c77251fea1a4deae25b26c42860372d2710a026be941728691ec62b5424\",\n    \"sizeBytes\": \"834\"\n  },\n  \"platform\": {\n    \"properties\": [{\n      \"name\": \"container-image\",\n      \"value\": \"reg.docker.alibaba-inc.com/apsara-citest/janus-union:go1.20\"\n    }]\n  }\n}"
1720948475.600905 [0 172.26.0.3:37702] "DEL" "Dispatching:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.601740 [0 172.26.0.3:37702] "RPOPLPUSH" "{06S}cpu" "{06S}cpu_dequeue"
1720948475.601938 [0 172.26.0.3:37702] "BRPOPLPUSH" "{06S}cpu" "{06S}cpu_dequeue" "1"
1720948475.605707 [0 172.26.0.3:37884] "SETEX" "Operation:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "604800" "{\n  \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n  \"metadata\": {\n    \"@type\": \"type.googleapis.com/build.buildfarm.v1test.ExecutingOperationMetadata\",\n    \"executeOperationMetadata\": {\n      \"stage\": \"EXECUTING\",\n      \"actionDigest\": {\n        \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n        \"sizeBytes\": \"224\"\n      },\n      \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n      \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\"\n    },\n    \"requestMetadata\": {\n    },\n    \"startedAt\": \"1720948475604\",\n    \"executingOn\": \"172.26.0.3:8981\"\n  }\n}"
1720948475.605919 [0 172.26.0.3:37884] "SADD" "" "shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.607158 [0 172.26.0.3:37884] "PING"
1720948475.607240 [0 172.26.0.3:37884] "PUBLISH" "OperationChannel:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "{\n  \"effectiveAt\": \"2024-07-14T09:14:35.605Z\",\n  \"source\": \"buildfarm-worker-172.26.0.3:8981-fc464f78-6432-428b-a7f0-a43d6aa038ee\",\n  \"reset\": {\n    \"expiresAt\": \"2024-07-14T09:14:45.605Z\",\n    \"operation\": {\n      \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n      \"metadata\": {\n        \"@type\": \"type.googleapis.com/build.bazel.remote.execution.v2.ExecuteOperationMetadata\",\n        \"stage\": \"EXECUTING\",\n        \"actionDigest\": {\n          \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n          \"sizeBytes\": \"224\"\n        },\n        \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n        \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\"\n      }\n    }\n  }\n}"
1720948475.614962 [0 172.26.0.3:37884] "SETEX" "Operation:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "604800" "{\n  \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n  \"metadata\": {\n    \"@type\": \"type.googleapis.com/build.buildfarm.v1test.CompletedOperationMetadata\",\n    \"executeOperationMetadata\": {\n      \"stage\": \"COMPLETED\",\n      \"actionDigest\": {\n        \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n        \"sizeBytes\": \"224\"\n      },\n      \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n      \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\"\n    },\n    \"requestMetadata\": {\n    }\n  },\n  \"done\": true,\n  \"response\": {\n    \"@type\": \"type.googleapis.com/build.bazel.remote.execution.v2.ExecuteResponse\",\n    \"result\": {\n      \"exitCode\": -1,\n      \"stderrDigest\": {\n        \"hash\": \"ab362a81c7f1c61775c0d8d105cd16d8a3c64f01c7c45343db38c8964051811e\",\n        \"sizeBytes\": \"75\"\n      },\n      \"executionMetadata\": {\n        \"worker\": \"172.26.0.3:8981\",\n        \"queuedTimestamp\": \"2024-07-14T09:14:35.582Z\",\n        \"workerStartTimestamp\": \"2024-07-14T09:14:35.601Z\",\n        \"workerCompletedTimestamp\": \"2024-07-14T09:14:35.611Z\",\n        \"inputFetchStartTimestamp\": \"2024-07-14T09:14:35.601Z\",\n        \"inputFetchCompletedTimestamp\": \"2024-07-14T09:14:35.603Z\",\n        \"executionStartTimestamp\": \"2024-07-14T09:14:35.607Z\",\n        \"executionCompletedTimestamp\": \"2024-07-14T09:14:35.610Z\",\n        \"outputUploadStartTimestamp\": \"2024-07-14T09:14:35.610Z\",\n        \"outputUploadCompletedTimestamp\": \"2024-07-14T09:14:35.611Z\"\n      }\n    },\n    \"status\": {\n      \"code\": 3\n    }\n  }\n}"
1720948475.615234 [0 172.26.0.3:37884] "SADD" "" "shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.617348 [0 172.26.0.3:37884] "PING"
1720948475.617454 [0 172.26.0.3:37884] "PUBLISH" "OperationChannel:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0" "{\n  \"effectiveAt\": \"2024-07-14T09:14:35.615Z\",\n  \"source\": \"buildfarm-worker-172.26.0.3:8981-fc464f78-6432-428b-a7f0-a43d6aa038ee\",\n  \"reset\": {\n    \"expiresAt\": \"2024-07-14T09:14:45.615Z\",\n    \"operation\": {\n      \"name\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0\",\n      \"metadata\": {\n        \"@type\": \"type.googleapis.com/build.bazel.remote.execution.v2.ExecuteOperationMetadata\",\n        \"stage\": \"COMPLETED\",\n        \"actionDigest\": {\n          \"hash\": \"37bcf85608dfcdfe8a7a263287a3357381845b6d5c0def165620ad90e61330a9\",\n          \"sizeBytes\": \"224\"\n        },\n        \"stdoutStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stdout\",\n        \"stderrStreamName\": \"shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0/streams/stderr\"\n      },\n      \"done\": true,\n      \"response\": {\n        \"@type\": \"type.googleapis.com/build.bazel.remote.execution.v2.ExecuteResponse\",\n        \"result\": {\n          \"exitCode\": -1,\n          \"stderrDigest\": {\n            \"hash\": \"ab362a81c7f1c61775c0d8d105cd16d8a3c64f01c7c45343db38c8964051811e\",\n            \"sizeBytes\": \"75\"\n          },\n          \"executionMetadata\": {\n            \"worker\": \"172.26.0.3:8981\",\n            \"queuedTimestamp\": \"2024-07-14T09:14:35.582Z\",\n            \"workerStartTimestamp\": \"2024-07-14T09:14:35.601Z\",\n            \"workerCompletedTimestamp\": \"2024-07-14T09:14:35.611Z\",\n            \"inputFetchStartTimestamp\": \"2024-07-14T09:14:35.601Z\",\n            \"inputFetchCompletedTimestamp\": \"2024-07-14T09:14:35.603Z\",\n            \"executionStartTimestamp\": \"2024-07-14T09:14:35.607Z\",\n            \"executionCompletedTimestamp\": \"2024-07-14T09:14:35.610Z\",\n            \"outputUploadStartTimestamp\": \"2024-07-14T09:14:35.610Z\",\n            \"outputUploadCompletedTimestamp\": \"2024-07-14T09:14:35.611Z\"\n          }\n        },\n        \"status\": {\n          \"code\": 3\n        }\n      }\n    }\n  }\n}"
1720948475.617957 [0 172.26.0.3:37884] "HDEL" "DispatchedOperations" "shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.624688 [0 172.26.0.4:40490] "UNSUBSCRIBE" "OperationChannel:shard/operations/03dbdd6a-4971-460b-a896-362a1bdc1cb0"
1720948475.860414 [0 172.26.0.7:36552] "BRPOPLPUSH" "{Arrival}:PreQueuedOperations" "{Arrival}:PreQueuedOperations_dequeue" "1"
1720948475.960793 [0 172.26.0.6:59486] "BRPOPLPUSH" "{06S}cpu" "{06S}cpu_dequeue" "1"
1720948476.427376 [0 172.26.0.7:36546] "LLEN" "{06S}cpu"
1720948476.427497 [0 172.26.0.7:36546] "HGETALL" "DispatchedOperations"
1720948476.438309 [0 172.26.0.6:35568] "HSET" "Workers_execute" "172.26.0.6:8981" "{\n  \"endpoint\": \"172.26.0.6:8981\",\n  \"expireAt\": \"1720948506437\",\n  \"workerType\": 3,\n  \"firstRegisteredAt\": \"1720944725698\"\n}"
1720948476.438399 [0 172.26.0.6:35568] "HSET" "Workers_storage" "172.26.0.6:8981" "{\n  \"endpoint\": \"172.26.0.6:8981\",\n  \"expireAt\": \"1720948506437\",\n  \"workerType\": 3,\n  \"firstRegisteredAt\": \"1720944725698\"\n}"
1720948476.551791 [0 172.26.0.4:40696] "LLEN" "{06S}cpu"
1720948476.551916 [0 172.26.0.4:40696] "HGETALL" "Disp98u验证87；了，民办v一天m mzxcghujhgfdswertyyuatchedOperations"
1720948476.663587 [0 172.26.0.4:34072] "BRPOPLPUSH" "{Arrival}:PreQueuedOperations" "{Arrival}:PreQueuedOperations_dequeue" "1"
1720948476.663664 [0 172.26.0.3:37702] "BRPOPLPUSH" "{06S}cpu" "{06S}cpu_dequeue" "1"
1720948476.864287 [0 172.26.0.7:36552] "BRPOPLPUSH" "{Arrival}:PreQueuedOperations" "{Arrival}:PreQueuedOperations_dequeue" "1"
```

- {06S}cpu：任务的主要调度队列，负责将任务分配给可用的处理单元。系统会定期检查队列长度（通过 LLEN），并从中选取任务进行处理（通过 BRPOPLPUSH）。
- {Arrival}:PreQueuedOperations：用于存储新到达、等待进一步处理的操作。初始任务接收队列，等待进入实际处理流水线或进行预处理的队列。
- {Arrival}:PreQueuedOperations_dequeue：用于存储正从 {Arrival}:PreQueuedOperations 转移过来的，准备被进一步处理的任务。任务的前置处理中间队列，表示这些任务已经通过初步检查并准备进行进一步的处理。
- OperationChannel:{operation_id}：用于在系统中广播特定操作的状态变化或进展。
1. **任务发布和初步接收**：
   - 一个新任务到达系统，被添加到 `{Arrival}:PreQueuedOperations` 队列中，并通过 `PUBLISH` 通知有关组件。
   - 该任务等待进一步的初步处理或检查，某个工作节点通过 `BRPOPLPUSH` 从 `{Arrival}:PreQueuedOperations` 队列中取出任务，放入 `{Arrival}:PreQueuedOperations_dequeue` 队列。
2. **任务前置处理**：
   - `{Arrival}:PreQueuedOperations_dequeue` 队列中的任务被处理并做相应的准备。
   - 准备好后，任务被再次检查，通过 `PUBLISH` 通知有关组件状态变化，并将准备好的任务推送到主处理队列 `{06S}cpu`。
3. **任务调度和处理**：
   - 在 `{06S}cpu` 队列中等待处理的任务被调度。
   - 通过 `BRPOPLPUSH` 将任务传递到 `{06S}cpu_dequeue` 队列，这表示任务已经被工作节点接收并开始处理。
   - 工作节点系统通过 `PUBLISH` 报告任务的执行状态（如执行、完成或失败等）。
4. **任务完成和清理**：
   - 处理完成后，任务从 `{06S}cpu_dequeue` 队列中删除（通过 `LREM`）。
   - 相关的哈希表记录（如 `DispatchedOperations`）也被更新或删除。
   - 最终通过 `PUBLISH` 将最终状态信息通知其他组件。
5. **工作节点心跳和状态更新**：
   - 工作节点定期通过 `HSET` 更新其状态和元数据（如 `Workers_execute` 和 `Workers_storage` 哈希表）。
## 预处理队列 {Arrival}:PreQueuedOperations
通过配置文件可以指定用于存储等待转换为 `QueryEntry`的`ExecuteEntry`的键名。
```yaml
backplane:
  preQueuedOperationsListName: "execute-entry"
```
`priorityQueue`参数也会影响最终的键名：
```java
  private static String getPreQueuedOperationsListName() {
    String name = configs.getBackplane().getPreQueuedOperationsListName();
    return createFullQueueName(name, getQueueType());
  }  

  private static Queue.QUEUE_TYPE getQueueType() {
    return configs.getBackplane().isPriorityQueue()
        ? Queue.QUEUE_TYPE.priority
        : Queue.QUEUE_TYPE.standard;
}

  private static String createFullQueueName(String base, Queue.QUEUE_TYPE type) {
    // To maintain forwards compatibility, we do not append the type to the regular queue
    // implementation.
    return ((!type.equals(Queue.QUEUE_TYPE.standard)) ? base + "_" + type : base);
  }
```
默认情况下，队列名为`"{Arrival}:PreQueuedOperations"`
![image.png](https://intranetproxy.alipay.com/skylark/lark/0/2024/png/140156364/1720947015204-dde2eabd-77c9-4536-b669-61a8087bf477.png#clientId=ud2541558-1a16-4&from=paste&height=137&id=DqzEL&originHeight=274&originWidth=1018&originalType=binary&ratio=2&rotation=0&showTitle=false&size=99769&status=done&style=none&taskId=u6a4941b3-0a8c-4fe8-b6b3-c36e360b957&title=&width=509)
## 任务调度队列 {06S}cpu
这里采用bf集群中利用promethues客户端暴露的运行指标来判断。集群向外提供了很多指标，我们使用grafana进行处理，并选择部分重点指标进行分析。
promethues联合grafana查看系统报表方法见[https://aliyuque.antfin.com/g/cloudstorage/devops/zn2hulmy6x93ehis/collaborator/join?token=2nLH4Rz9DhrLxsrS&source=doc_collaborator#](https://aliyuque.antfin.com/g/cloudstorage/devops/zn2hulmy6x93ehis/collaborator/join?token=2nLH4Rz9DhrLxsrS&source=doc_collaborator#) 《使用Brafana 可视化系统监控信息》
我们监控这样一段时间的指标：使用ptest工具[@四水(xumiaoyong.xmy)](/xumiaoyong.xmy)，向集群发送一批长时间运行的任务：
```shell
[root@j63e03474.sqa.eu95 /home/jiaomian.wjw/pcc]
#cat sleep.c 
#include <stdio.h>
#include <unistd.h> // 包含sleep函数的头文件

int main() {
    printf("Hello, world!123\n");
    sleep(20); // 程序睡眠20秒
    printf("Waking up after 20 seconds!\n");
    return 0;
}


[root@j63e03474.sqa.eu95 /home/jiaomian.wjw/pcc]
#gcc -o sleep sleep.c 

[root@j63e03474.sqa.eu95 /home/jiaomian.wjw/pcc]
#ptest ./sleep -c 500 --look_for_action_cache=false
```
这里让500个任务在worker中sleep20s，以模仿长时间的构建任务。随后，查看这段时间的系统监控如下：
![image.png](https://intranetproxy.alipay.com/skylark/lark/0/2024/png/140156364/1721974640839-a6bf6a1e-da1e-401c-a07d-b292b672c94a.png#clientId=uc2db8af6-acf0-4&from=paste&height=665&id=ue1983599&originHeight=1330&originWidth=2934&originalType=binary&ratio=2&rotation=0&showTitle=false&size=796460&status=done&style=none&taskId=u05b08578-0d20-47c5-99fe-64e9a01c989&title=&width=1467)
将指标分为几组，分别分析：

1. worker指标
   - 白线表示worker个数。其始终为1，证明了集群中当前只有一个工作节点。
2. 系统线程指标
   - 红线表示当前jvm的线程数。

可以看到其在任务启动后，从8提高并保持在了70左右，这些线程在单个worker上提供了任务的并行能力。

3. 实际任务指标
   - 蓝线表示cpu队列大小。
   - 黄线是处于排队状态的任务的累积值。
   - 紫线是处于执行状态的任务的累积值。
   - 绿线是处于完成状态的任务的累积值。

可以看到，cpu队列大小在任务启动后，提升到了一个高值（424），随后逐渐降低至0；处于排队数的任务从一开始就达到500；处于执行和完成的任务数均以64为跨度逐渐升高，且完成的任务曲线较执行任务的有延迟（20s）。
**总结这些指标，我们可以得出cpu队列表示了当前实时排队任务数的结论。**
```
operation_exit_code_total{exit_code="1",} 4.0
operation_exit_code_total{exit_code="127",} 5.0
operation_exit_code_total{exit_code="139",} 8.0
operation_exit_code_total{exit_code="0",} 3067.0
```
total=3084
```
operations_stage_load_total{stage_name="EXECUTING",} 3248.0
operations_stage_load_total{stage_name="COMPLETED",} 3084.0
operations_stage_load_total{stage_name="QUEUED",} 3282.0
operations_stage_load_total{stage_name="UNKNOWN",} 3533.0
```
-------
```
operations_stage_load_total{stage_name="CACHE_CHECK",} 14.0
operations_stage_load_total{stage_name="EXECUTING",} 3816.0
operations_stage_load_total{stage_name="COMPLETED",} 3716.0
operations_stage_load_total{stage_name="QUEUED",} 3851.0
operations_stage_load_total{stage_name="UNKNOWN",} 4101.0
```
```
operation_exit_code_total{exit_code="1",} 4.0
operation_exit_code_total{exit_code="127",} 5.0
operation_exit_code_total{exit_code="139",} 8.0
operation_exit_code_total{exit_code="0",} 3699.0
```
total=3716

```
public @Nullable String poll(UnifiedJedis unified) {
  String queue = queues.get(roundRobinPopIndex());
  try (Jedis jedis = getJedisFromKey(unified, queue)) {
    return queueDecorator.decorate(jedis, queue).poll();  // 此处对应LMOVE
  }
}
```
![image.png](https://intranetproxy.alipay.com/skylark/lark/0/2024/png/140156364/1722307052610-42285392-9790-471b-b5c5-0b40ccc5dbd4.png#clientId=u8becd60b-eb11-4&from=paste&height=551&id=ud67634f9&originHeight=1102&originWidth=2544&originalType=binary&ratio=2&rotation=0&showTitle=false&size=734965&status=done&style=none&taskId=u5b73be46-2386-4e1e-976c-b78bb685672&title=&width=1272)
