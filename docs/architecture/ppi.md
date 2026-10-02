# Payment Provider Interface
This module is where all communications with third-party platforms happen. We implemented the adapters for all the supported payment providers. 

## Payment Workflow
Once an invoice is finallized and ready to be paid for, the system initiates a payment attempt to the pre-configured payment provider. This attemp it not to actually pay the payment per se. But to get a checkout URL from the provider in question. And if the attemp was succefull and the PPI gets a checkout URL, the URL will be emailed to the user, so that the user can authorize and finish the payment. 

Having accepted the payment authorization from the user, the payment provider sends back a webhook to our server confirming the user's payment, at which point the payment provider's adapter takes over and parses the webhook. If the webhook was indeed a legitimate one, the adapter sends the payment succeful message to our broker.

Different parts of our codebase react to a payment event, among which invoice is the main one. Since the invoice, which was earlier finalized, needs a succeful payment to be labeled `paid`, it needs to subscribe to payment succeful messages. So invoice module, or any other module, would call `RegisterModuleSubscriber` in ppi/subscriber to register their functions that are supposed to react to any payment message.