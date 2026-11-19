-- name: CheckSchema :one
SELECT (to_regnamespace('outguessed') IS NOT NULL)::boolean AS ready;
