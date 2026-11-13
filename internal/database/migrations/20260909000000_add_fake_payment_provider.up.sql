INSERT INTO payment_provider (payment_provider_code, description)
VALUES ('fake', 'Development payment provider')
ON CONFLICT (payment_provider_code) DO NOTHING;
