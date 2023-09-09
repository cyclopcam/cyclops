# AppUI

This is the UI for the mobile app. We don't do any of the actual camera
monitoring here. The duties handled by this UI are things like connecting to new
servers, and switching between servers. Also, you can scan your LAN for new
servers.

# How to dev

If you run `npm run dev` and point localhost at this page, you should see it
operate in debug mode.

-   To debug the welcome screen, set registeredFakeServers to [], inside
    nattypes.ts
