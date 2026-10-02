# Overview
This application is a standalone and self-contained binary that handles all the billing related things for any platform. Its main problem that it purports to solve is to make businness platforms be able to support all payment providers without any implementation overhead. All businesses need to do is bring their credentials for supported providers and the rest will be handled by the program running in its own environment.

## How it Works
After hosting the backend and setting things up, the platform sends a request to billing server to create an account for all its users. The billing server only needs a tracking ID from the platform along with some metadata about the user. Once the user account is created, the billing server manages subscriptions, generating invoice, communicating with the payment providers and .

## Main Capabilities
- Integrate once with the server and support major providers instantly
- Support reccuring payments even if the providers do not have it 
- Own your own data and keep your sovereignty over your data 
- compatible with any businness platform