INSERT INTO partners (name) VALUES ('mattilsynet');
INSERT INTO environments(partner_id, name) VALUES ((SELECT id FROM partners LIMIT 1), 'dev');