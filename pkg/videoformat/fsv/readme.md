# Filesystem Video (FSV)

FSV is a formalization of a directory/file hierarchy, with associated naming
convention, for storing video files. We also manage the chunking of long
recording streams into smaller files, and also the rolling over of files so that
we don't exceed the storage budget.

## Use of rf1 data types

We borrow some rf1 data types such as rf1.NALU for convenience, but we maintain
an abstraction interface between fsv and rf1, so that it should be easy to
switch to a different video file format if necessary.

## Archive size and index/file overhead

Should we keep an in-memory index of all files in the archive?

The pros:

1. We can maintain an ordered list, so finding all files that span a given time
   window is a trivial binary search.
2. No need to ask the OS every time.
3. (Ancillary benefit) because we don't have to ask the OS to list files every
   time we do a read, we can keep the naming convention extremely simple - i.e.
   one directory per stream.

The cons:

1. We have to maintain an index in memory, which uses memory.
2. If we ever wanted to implement independent reader and writer processes, then
   this would complicate that.

How much memory? Firstly, our data structure would need to store the filename
(eg "1708584695_video.rf1i"), the start time and duration. Let's say 40 bytes
per file. Let's imagine a 7 day archive, with one file for every 1000 seconds.
7 \* 24 \* 3600 = 604800 seconds. 604800 / 1000 = 604 files. 604 \* 40 = 24160
bytes per stream. This is small enough that the pros seems to outweight the
cons.

Update -- because of our rf1 multi-file architecture, we also need to store the
list of tracks inside this index. So the data structure is more like 80 bytes
per file. Even if we double the 24kb per stream, that's still nothing.

## Write Buffer

I'm experimenting with adding a per-stream write buffer into the archive. What
got me to try this is that I'm experiencing extremely slow write times on an
NTFS USB spinning disc drive, mounted to a WSL VM. I've tried enabling write
caching in Windows for this disk, but it doesn't make a difference. It seems to
be limited to about 2 MB/s. By caching writes, we can bunch up the writes to
each stream. Let's see if this makes a difference.

Yes, it does seem to make a difference, at least on my problematic USB drive.

## Write Buffer v2

During testing of a cheap 128GB USB stick (FAT32), plugged into a Raspberry Pi
5, I saw a lot of dropped video packets. This is probably attributable to either
the NN processing thread, or the slow disk writes to the FSV archive, and I
suspect it is the latter. It seems irresponsible NOT to allow asynchronous
writes of the video feeds, so I'm going to do that. If this mode is enabled,
then writes always occur from a background thread. OK - I ended up making
buffered writes _always_ operate asynchronously.
