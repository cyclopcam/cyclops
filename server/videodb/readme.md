# Video DB

This is my 2nd attempt at creating the recording database.

The 1st attempt was centered around the idea that there wouldn't be many
recordings, and with an emphasis on labeling.

Here I'm focusing first on continuous recording. We need to be efficient when
there are hundreds of hours of footage. The user must be able to scan through
this footage easily, and we don't want to lose any frames.

## DB Design

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

| Frames per Segment | Index Bits | Max Segment Size | Max Average Frame Size |
| ------------------ | ---------- | ---------------- | ---------------------- |
| 1024               | 30         | 1 GB             | 1 MB                   |
| 1024               | 31         | 2 GB             | 2 MB                   |
| 1024               | 32         | 4 GB             | 4 MB                   |

An H264 video stream of about 1920 x 1080 has an average frame size of under 100
KB, so an average frame size of 1MB is a lot. If we run out of space in a
segment, we can just close it and start another. Let's do 30 bits for he
location.

Adding the frame location and time bits: 30 + 22 = 52 bits, leaving 12 bits for
flags such as the NALU type.
