# Video DB

This is my 2nd attempt at creating the recording database.

The 1st attempt was centered around the idea that there wouldn't be many
recordings, and with an emphasis on labeling.

Here I'm focusing first on continuous recording. We need to be efficient when
there are hundreds of hours of footage. The user must be able to scan through
this footage easily, and we don't want to lose any frames.

## Video Archive

All the video footage is stored inside our 'fsv' format archive. Initially,
we'll just be supporting our own 'rf1' video format, but we could conceivably
support more mainstream formats such as mp4, if that turns out to be useful.

The primary reason for using rf1 is that it is designed to withstand a system
crash, and still retain all the video data that was successfully written to
disk. The second reason is efficiency. 'rf1' files are extremely simple - we're
just copying the raw NALUs to disk.

## Database Design

The `event` table is fat. The `objects` field is a JSON object that contains up
to 5 minutes of object box movement, or 10KB of data, whichever limit comes
first.

The `event_summary` table is lean, and used to draw the colored timeline of a
camera, where the colors allow the user to quickly tell which parts of the video
had interesting detections.

The `event_summary` table stores information in fixed-size time slots. For
example, if the time slot is 5 minutes, then there would be one record per
camera, for every 5 minute segment of time. If there are zero events for a 5
minute segment, then we don't bother storing a record. But how do we know that 5
minutes is the ideal interval? The problem we're trying to solve here is quickly
drawing a timeline below a camera that shows the points in time where
interesting events occurred. Our average data usage in our `event` table is 12KB
for 300 frames. At 10 FPS, that is 12KB for 30 seconds. If we make our
`event_summary` segment size too large (eg 1 hour), then we'll end up having
poor resolution on our timeline.

The resolution is really the constraint here, and let's work conservatively, and
say that the person has a 4K monitor. A single pixel on the timeline is probably
not visible enough, but two pixels will be fine. So let's say we target a
resolution of 2000 pixels on the timeline. That's 2000 segments. 2000 segments
at 5 minutes per segment is 166 hours, or 7 days. That seems like a decent
zoomed-out view, in terms of performance/quality tradeoff. But what happens when
we try to zoom in, say to 2 days? 2 days split across 2000 pixels is 1.44
minutes per pixel. And what about 2 hours? 2 hours split across 2000 pixels is
3.6 seconds per pixel. This is a huge dynamic range, and it's making me wonder
if we should just do a hierarchical (eg power-of-2 sizes) event summary table,
like mip-maps.

Yes, I think we do hierarchical summaries - anything else will be splitting
hairs, or reaching bad performance corner cases when zoomed in or zoomed out.

## Event Summary Bitmaps

After considering this for some time, it occurred to me that we might as well
represent the event summary as a bitmap. Imagine a bitmap that is 2048 wide and
32 high. The 2048 columns (X) are distinct time segments. The 32 rows (Y) are 32
different items that we're interested in, such as "person", "car", etc. 32 is a
lot of object types, so we choose that to be conservative. A key thing is that
these rows can literally be bitmaps - i.e. we only need a single bit to say
whether such as object was found during that time segment. It's pretty obvious
that this data will compress well. Even with zero compression, we still have a
tiny amount of data per segment. At 2048 x 32, we have 8KB raw. Assuming we get
a 10:1 compression ratio, that's 800 bytes per time segment. Even at a
compression ratio of 5:1, we can almost fit into a single network packet.

I love this pure bitmap representation, because we no longer have to fuss over
wasted space in our SQLite DB, or efficiency, or worst case. In addition, mipmap
tiles are dead simple to reason about. The only thing that remains is to pick
the lowest mip level, and the tile size. Here's a table showing some candidate
numbers.

-   Segment: The duration of each time segment, at the finest granularity
-   Size: The number of segments per tile
-   Tile Duration: Duration of a full tile at the finest granularity
-   Raw Size: Raw size of bitmap, if capable of holding 32 object types

| Segment | Size | Tile Duration | Raw Size | Compressed Size @ 5:1 |
| ------- | ---- | ------------- | -------- | --------------------- |
| 1s      | 512  | 8.5m          | 2KB      | 409 bytes             |
| 1s      | 1024 | 17m           | 4KB      | 819 bytes             |
| 1s      | 2048 | 34.1m         | 8KB      | 1638 bytes            |
| 2s      | 512  | 17m           | 1KB      | 204 bytes             |
| 2s      | 1024 | 34.1m         | 2KB      | 409 bytes             |
| 2s      | 2048 | 68.3m         | 4KB      | 819 bytes             |

At first it seems tempting to make the tile size large (1s, 2048 wide), but the
problem with that, is that we have to wait 34 minutes before our latest tile is
created. If the user wants an event summary during that time, somebody (either
server or client) will need to synthesize it from the dense event data, which
can be on the order of megabytes for half an hour.

But hang on! We're expected to be running 24/7, so it should be easy for us to
maintain an up to date tile by directly feeding our in-memory events to the
tiler, in real-time. There's no need to roundtrip this stuff through the
database. So then there's no longer any consideration of liveness, or
performance overhead to build an up-to-date tile. The only thing that remains is
to decide on the finest granularity. I think we might as well do tiles that are
1024 pixels wide, at 1 second per pixel granularity, because those are such nice
round numbers.

## Building Higher Level Tiles

The following diagram represents tiles being built. The dashes are moments that
have already passed, and the x's are in the future. The blank portions are
regions of time when the system was switched off.

    Level 4 |---------------------------------------------------------xxxxxxxxxxxxxxxxxxxxxx|
    Level 3 |---------------------------------------|-----------------xxxxxxxxxxxxxxxxxxxxxx|
    Level 2 |-------------------|-------------------|-----------------xx|
    Level 1 |---------|---------|---------|---------|         |-------xx|
    Level 0 |----|----|----|----|----|----|----|----|--- |    |----|--xx|

Before going on, let's consider how many levels we need in practice. Our lowest
level (level 0) has one pixel per second. Let's use a screen with resolution of
2000 horizontal pixels as our representative use case. This would be on the
desktop. A phone screen would be much less (eg 300 horizontal CSS pixels).

Let's consider various tile levels and the time spans that they would represent
on a 2000 pixel wide screen. Each level is 2x the number of seconds/pixel of the
previous level.

| Level | Pixels | Seconds/Pixel | Duration |
| ----- | ------ | ------------- | -------- |
| 0     | 2000   | 1             | 33.3m    |
| 1     | 2000   | 2             | 1.1h     |
| 2     | 2000   | 4             | 2.2h     |
| 3     | 2000   | 8             | 4.4h     |
| 4     | 2000   | 16            | 8.8h     |
| 5     | 2000   | 32            | 17.7h    |
| 6     | 2000   | 64            | 1.5d     |
| 7     | 2000   | 128           | 2.9d     |
| 8     | 2000   | 256           | 5.9d     |
| 9     | 2000   | 512           | 11.8d    |
| 10    | 2000   | 1024          | 23.7d    |

The bottom line is that we need to support going up to many levels. On a phone,
it's likely to be even more levels, because of the reduced resolution.

It's clear that we need a mechanism which continuously builds higher level tiles
in the background, so that they're ready for consumption at any time. If our
dataset was generated once-off, then this is a trivial. However, our job is
slightly more complicated, because our tiles are constantly being generated.

I'm thinking right now, that if we just do this one thing, everything will be
OK: If a tile still has any portion of itself in the future, then we don't write
it to disk. Whenever a caller requests tiles that aren't in the DB yet, we build
them on the fly, from lower level tiles. I can't tell for sure, but it looks to
me like the time to build up tiles like this should O(max_level). Our levels
won't get higher than about 10, and merging/compacting bitmaps should be very
fast. Let's see!
