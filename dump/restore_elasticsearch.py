#!/usr/bin/env python3
"""
Restore Elasticsearch dari search.json (hasil export search API).
Usage: python restore_elasticsearch.py [--url http://localhost:9201]
"""
import json
import argparse
import urllib.request
import urllib.error

def restore_elasticsearch(json_path: str, es_url: str):
    with open(json_path, "r", encoding="utf-8") as f:
        data = json.load(f)

    hits = data.get("hits", {}).get("hits", [])
    if not hits:
        print("Tidak ada data di search.json")
        return

    # Build bulk request (NDJSON format)
    bulk_lines = []
    for hit in hits:
        index_name = hit.get("_index", "franchises")
        doc_id = hit.get("_id")
        source = hit.get("_source", {})

        action = {"index": {"_index": index_name, "_id": doc_id}}
        bulk_lines.append(json.dumps(action))
        bulk_lines.append(json.dumps(source))

    bulk_body = "\n".join(bulk_lines) + "\n"
    url = f"{es_url.rstrip('/')}/_bulk"

    req = urllib.request.Request(
        url,
        data=bulk_body.encode("utf-8"),
        method="POST",
        headers={"Content-Type": "application/x-ndjson"},
    )

    try:
        with urllib.request.urlopen(req) as resp:
            result = json.loads(resp.read().decode())
            if result.get("errors"):
                print("Beberapa dokumen gagal:")
                for item in result.get("items", []):
                    if "index" in item and item["index"].get("error"):
                        print(f"  - {item['index'].get('error')}")
            else:
                print(f"Berhasil restore {len(hits)} dokumen ke Elasticsearch")
    except urllib.error.URLError as e:
        print(f"Error: {e}")
        if hasattr(e, "read"):
            print(e.read().decode())

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="http://localhost:9201", help="Elasticsearch URL")
    parser.add_argument("--file", default="search.json", help="Path ke search.json")
    args = parser.parse_args()

    restore_elasticsearch(args.file, args.url)
