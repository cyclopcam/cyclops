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
