#!/usr/bin/env python3

import argparse
import gzip
import io
import json
import sys

import yaml


# https://raw.githubusercontent.com/jmoiron/humanize/master/humanize/filesize.py
suffixes = {
    'decimal': ('kB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'),
    'binary': ('KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB', 'ZiB', 'YiB'),
    'gnu': "KMGTPEZY",
}


def naturalsize(value, binary=False, gnu=False, format='%.1f'):
    """Format a number of byteslike a human readable filesize (eg. 10 kB).  By
    default, decimal suffixes (kB, MB) are used.  Passing binary=true will use
    binary suffixes (KiB, MiB) are used and the base will be 2**10 instead of
    10**3.  If ``gnu`` is True, the binary argument is ignored and GNU-style
    (ls -sh style) prefixes are used (K, M) with the 2**10 definition.
    Non-gnu modes are compatible with jinja2's ``filesizeformat`` filter."""
    if gnu: suffix = suffixes['gnu']
    elif binary: suffix = suffixes['binary']
    else: suffix = suffixes['decimal']

    base = 1024 if (gnu or binary) else 1000
    bytes = float(value)

    if bytes == 1 and not gnu: return '1 Byte'
    elif bytes < base and not gnu: return '%d Bytes' % bytes
    elif bytes < base and gnu: return '%dB' % bytes

    for i,s in enumerate(suffix):
        unit = base ** (i+2)
        if bytes < unit and not gnu:
            return (format + ' %s') % ((base * bytes / unit), s)
        elif bytes < unit and gnu:
            return (format + '%s') % ((base * bytes / unit), s)
    if gnu:
        return (format + '%s') % ((base * bytes / unit), s)
    return (format + ' %s') % ((base * bytes / unit), s)


def summarize(p, include=None):
    for k, v in p.items():
        if not v:
            continue
        if include and k not in include:
            continue

        print(k, end=': ')
        if isinstance(v, str):
            # scalar
            print(v)
        elif isinstance(v, list):
            print(len(v), "elements")
        else:
            print(v)


def analyze(payload):
    # dump again, to clean up json formatting
    raw = json.dumps(payload, separators=(',', ':')).encode('utf-8')
    raw_gz = gzip.compress(raw)
    print("{} ({}) in ({} ({}) gz)".format(len(raw), naturalsize(len(raw)), len(raw_gz), naturalsize(len(raw_gz))))

    #print("** service")
    #summarize(payload['service'])
    if 'transactions' in payload:
        print("** {} transactions".format(len(payload['transactions'])))
        for t in payload['transactions']:
            summarize(t, include=['spans'])
            if "spans" in t:
                for s in t['spans']:
                    summarize(s, include=['stacktrace'])


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('raw', type=argparse.FileType(mode='r'))
    args = parser.parse_args()

    # load
    profile = yaml.load(args.raw)
    for target in profile['loadbeat']['targets']:
        print("type: {}, concurrent: {}, qps: {}".format(target['url'], target['concurrent'], target['qps']), end=' - ')
        if 'body' not in target:
            continue
        payload = json.loads(target['body'])
        analyze(payload)


if __name__ == '__main__':
    main()
