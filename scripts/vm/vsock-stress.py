#!/usr/bin/env python3
"""Run on a Linux VSOCK host against a guest agent on the given CID and port."""

import argparse
import collections
import concurrent.futures
import json
import socket
import time


def probe(cid, port, index):
    with socket.socket(socket.AF_VSOCK, socket.SOCK_STREAM) as conn:
        conn.settimeout(5)
        conn.connect((cid, port))
        # Include peers that close before a request and midway through a frame.
        if index % 10 == 0:
            return "closed_before_request"
        conn.sendall(b"\x04")
        if index % 10 == 1:
            return "closed_during_request"
        # A delayed payload forces the guest to retry an empty nonblocking read.
        time.sleep(0.001)
        conn.sendall(b"\x08\x01\x12\x00")
        response = bytearray()
        while len(response) < 5:
            chunk = conn.recv(5 - len(response))
            if not chunk:
                raise RuntimeError(f"incomplete ping response: {response!r}")
            response.extend(chunk)
        if response != b"\x04\x08\x01\x1a\x00":
            raise RuntimeError(f"invalid ping response: {response!r}")
        return "ping"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("cid", type=int)
    parser.add_argument("port", type=int, nargs="?", default=1024)
    parser.add_argument("--requests", type=int, default=10000)
    parser.add_argument("--workers", type=int, default=32)
    args = parser.parse_args()
    if args.requests <= 0 or args.workers <= 0:
        parser.error("requests and workers must be positive")
    start = time.monotonic()
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
        results = collections.Counter(pool.map(lambda i: probe(args.cid, args.port, i), range(args.requests)))
    # The listener must still work after the burst and a short idle interval.
    time.sleep(0.1)
    probe(args.cid, args.port, 2)
    print(json.dumps({"requests": args.requests, "seconds": round(time.monotonic() - start, 3), "results": dict(results)}))


if __name__ == "__main__":
    main()
