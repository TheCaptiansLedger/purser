-- Move item_people records for movie and book items to entry_people on their
-- parent library entry. Cast and authors belong to the title, not the file.
INSERT INTO entry_people(library_entry_id, person_id, role, start_date, end_date)
SELECT i.library_entry_id, ip.person_id, ip.role, '', ''
FROM item_people ip
JOIN items i ON i.id = ip.item_id
JOIN library_entries le ON le.id = i.library_entry_id
WHERE le.kind IN ('movie', 'book')
ON CONFLICT(library_entry_id, person_id, role) DO NOTHING;

DELETE FROM item_people
WHERE item_id IN (
    SELECT i.id FROM items i
    JOIN library_entries le ON le.id = i.library_entry_id
    WHERE le.kind IN ('movie', 'book')
);
