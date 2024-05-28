# Internal Code Notes

## Wake Intervals

The system has quite a few threads who's job is to wake up periodically and do
some task. I like to make the wake intervals randomish prime numbers, to reduce
the chance of different threads waking up at the same time and causing a spike
in CPU usage. It's better to spread such jobs out over time.
