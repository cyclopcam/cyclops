# Annex-B performance hit

On a Raspberry Pi 5, our Annex-B encoder (the bit that adds the Emulation
Prevention Byte) can encode 716 MB/s. Memcpy on this platform is 4690 MB/s.
Decode is 916 MB/s.

You can use misc_test.cpp to measure the speed yourself (instructions at top of
that file).

I don't have enough numbers right now to figure out the total system impact, but
my gut doesn't like it. It seems plausible that one should be able to improve
the speed of the encoder, but I don't know how. The alternative that I'm
considering is to delay encoding to Annex-B for as long as possible - perhaps
even doing it in the browser immediately before display.

If we're recording to disk, then it would be useful to avoid this penalty
completely, but that precludes us from using regular video formats like mp4. On
the other hand, we might want to avoid regular formats anyway.
