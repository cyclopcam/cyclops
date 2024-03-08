# Recording Format 1

This is my attempt at storing frames on disk in an efficient and simple manner.
Our goals here are to minimize recording overhead, and also provide for fast
seeking.

Limits:

    Maximim video time: 1024 seconds
    Maximum video size: 1 GB

## Index

The index starts with a 32-byte header, followed by an 8-byte entry for each
frame.

The 8-byte entry is layed out as follows:

| Bits  | Number of bits | Description      |
| ----- | -------------- | ---------------- |
| 63-42 | 22             | Time (PTS)       |
| 41-12 | 30             | Location on disk |
| 11-0  | 12             | Flags            |

Let's say we're storing 30 FPS of low res video. In this case we're limited by
our time limit of 1024 seconds. Our index size in such a case is 1024 \* 30 \* 8
= 240 KB.

## Sizes and File Counts

Our benchmark for a low res stream is 320 x 240, at 1000 bytes per frame
average, at 10 FPS. That yields 10000 bytes per second. At our time limit of
1024 seconds, that results in a NALU payload file of 10 MB, and an 80 KB index
file. If we record 24/7, and expect our media to cycle every 30 days, how many
files is that?

        30 * 24 * 3600 / 1024 = 2532 files

That seems like a reasonable number of files for one month of storage. Our
benchmark case for a high res stream is 100 KB average bytes per frame. At 10
FPS, this is 1MB per second. 1000 seconds brings us to 1GB, which is our size
limit. So the number of files per month will be in the same ballpark for high
res streams as for low res streams.

## Durability/Consistency

The original idea behind this file format was making it extremely easy to read
and write, and in particular, avoiding any need for a finalization phase. In
other words, to append to a file, you just write your packets, and you're done.
However, the caveat here is that we don't use any filesystem tricks or fsync to
ensure that the packets and index files are written in order, or written
atomically as a pair. If the operating system (or hardware) crashes during a
write, then you could end up with such inconsistencies (eg index entries
pointing to non-existent packets).
