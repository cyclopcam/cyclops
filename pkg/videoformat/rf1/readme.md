# Recording Format 1

This is my attempt at storing frames on disk in an efficient and simple manner.
Our goals here are to minimize recording overhead, and also provide for fast
seeking.

Limits:

    Maximum video time: 1024 seconds
    Maximum video size: 1 GB
    Maximum frames: 65535

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

## 16-bit Index Count

Inside the header of our index file, we store a 16-bit value that tells us how
many packets the file has. 16 bits seems like a small number for this day and
age, but is a limiting factor here? If we consider that our maximum number of
seconds is 1024, and imagine a high framerate of 30 (most systems use 10 fps),
then we get 1024 \* 30 = 30720 frames before we hit our 1024-second limit. This
number is well within the 16-bit limit of 65535.

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

## Avoiding Fragmentation

I didn't consider fragmentation initially, but this turns out to be a massive
issue, especially because we'll often be writing to good old hard drives. By the
nature of the recorder, we're writing tiny bits of data to many files in
sequence. This inevitably leads to fragmentation. We have slightly different
approaches to fragmentation avoidance in our index and packet files.

Index Files

To avoid fragmentation in our index files, we pre-size the file before writing
into it. The unwritten space will contain zeros. We know that these zeros are
not valid packet indices because a real packet index cannot be all zeros. The
'location' field of a 64-bit index entry cannot be zero, except for the very
first packet, which we make allowance for.

On my desktop linux machine, if I take zero measures to reduce fragmentation,
and record on 3 files simultaneously, I get 8MB blocks in the HD camera packet
files, and 4096 byte blocks in the index files.

## Dumb Pile of Frames

Initially, I didn't think about whether to encode packets in RBSP or Annex-B.
Later on, I realized that by storing packets encoded in Annex-B format with
start codes, our packet files become a playable video archive, without any
additional work. This is helpful for forensic type applications, where you want
to throw a possibly corrupted file at something like VLC and just see every
frame that you get out.

I'm not yet sure whether I'll encode to Annex-B if the incoming camera stream is
RBSP, because the encoding time is not zero. For example, if we were to encode
10 MB/s on an Rpi5, that would take 0.7% of total CPU time. This is probably
negligible enough that we should always just do it.

## Initial Design Thought Process / Notes / Scribbles

I'm going to prioritize efficiency over robustness in the face of a power
failure. It's easy to add more frequent disk flushes later on.

I think we want to split recordings up into files of a limited size. I'll call
these "segments".

Let's consider a segment for an HD recording, average frame size 100 KB, at 10
FPS. That's 1 MB per second, 60 MB per minute, 3.6 GB per hour.

A segment is made of two files: an index and a frame file. The index contains
locations in the frame file of the start of each frame, as well as flags
indicating the type of frame. The frame file contains the raw frame data. It's
really just a dump of the NALUs from the video stream, but we make sure to
include the most recent SPS and PPS NALUs at the start of each segment.

A 1GB segment would contain 1024 seconds, which is 17 minute of footage, and
10240 frames. If we store frame locations as 32-bit integers, that's 10240 \* 4
= 40 KB for the index. In addition to the frame location, we also need some bits
for flags, such as the NALU type. We're close to the 32-bit limit, so might as
well make the index 64-bit integers, which makes the index 80 KB.

Let's consider a low res recording (eg 320 x 240), with an average of 1000 bytes
per frame. 1024 seconds would be 1 MB. The index would still be 80 KB, which is
now adding 8%. This seems wasteful, but let's not complicate this further.

Each segment has a record in the database, with its start and end time. At a max
of 1024 seconds per second, that's 84 segments per day, 590 per week, 2615 per
month. These are fine numbers for DB record counts. We could do full table scans
on record counts like that, but we'll keep an index on start time.

But hang on! The byte offsets are not the only thing we need to store in the
index. We also need to store the frame presentation time (PTS). Video codecs do
this properly, and store the frame times as rational fractions. But we'll just
be lazy and store them as real numbers. Since we have at most 1024 seconds, we
need 10 bits to get to second-level precision. Obviously we need more precision
than that for video, so if we add another 12 bits, we get 1/4096 second
precision. That's 22 bits total for time.

Next question: How many bits do we need for the NALU location on disk?

| Seconds | Index Bits | Max Segment Size | Max Average Frame Size |
| ------- | ---------- | ---------------- | ---------------------- |
| 1024    | 30         | 1 GB             | 1 MB                   |
| 1024    | 31         | 2 GB             | 2 MB                   |
| 1024    | 32         | 4 GB             | 4 MB                   |

An H264 video stream of about 1920 x 1080 has an average frame size of under 100
KB, so an average frame size of 1MB is a lot. If we run out of space in a
segment, we can just close it and start another. Let's do 30 bits for he
location.

Adding the frame location and time bits: 30 + 22 = 52 bits, leaving 12 bits for
flags such as the NALU type.
