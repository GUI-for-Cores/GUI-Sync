# gui-sync

Synchronization configuration, supports `GUI.for.Clash` and `GUI.for.SingBox`.

## How to use

Prerequisites: A server: Windows or Linux

1、Download the program from the Release page to the server.

2、Run

```bash
# http
GUI-Sync --address 0.0.0.0 --port 8080 --token "A unique string"

# https
GUI-Sync --address 0.0.0.0 --port 8080 --token "A unique string" --cert /path/to/serer.cert --key /path/to/server.key
```

Note: The `token` needs to be consistent with the client.

## Directory Structure

The file storage structure is as follows:

```bash
data/
├── gfc
│   ├── id1.json
│   └── id2.json
└── gfs
    └── id1.json
```

## API

1、Query backup list using tag

```bash
GET /backup?tag=tag1

Authorization: Bearer token
User-Agent: GUI.for.Cores
```

Return data format

```json
["id1", "id2", "id3"]
```

2、Delete backup using tag and ids

```bash
DELETE /backup?tag=tag1&ids=id1,id2,id3

Authorization: Bearer token
User-Agent: GUI.for.Cores
```

3、Add backup using tag, id and files

```bash
POST /backup

Authorization: Bearer token
User-Agent: GUI.for.Cores

{
  "id": "id1",
  "tag": "tag1",
  "files": {
    "file1": "file content",
    "file2": "file content",
    "file3": "file content"
  }
}
```

4、Query backup details using tag and id

```bash
GET /sync?tag=tag1&id=id1

Authorization: Bearer token
User-Agent: GUI.for.Cores
```

Return data format

```json
{
  "id": "id1",
  "tag": "tag1",
  "files": {
    "file1": "file content",
    "file2": "file content",
    "file3": "file content"
  }
}
```
