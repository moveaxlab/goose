-- +goose up

CREATE OR REPLACE TYPE insert_repository_result AS object (v_owner_id NUMBER(38), v_repo_id NUMBER(38));

-- +goose statementbegin
CREATE OR REPLACE FUNCTION insert_repository(
    p_repo_full_name NVARCHAR2,
    p_owner_name NVARCHAR2,
    p_owner_type NVARCHAR2
) RETURN insert_repository_result IS result insert_repository_result;
BEGIN
    -- Check if the owner already exists
    SELECT owner_id INTO result.v_owner_id
    FROM owners
    WHERE owner_name = p_owner_name AND owner_type = p_owner_type;

    -- If the owner does not exist, insert a new owner
    IF result.v_owner_id IS NULL THEN

        INSERT INTO owners (owner_name, owner_type)
        VALUES (p_owner_name, p_owner_type)
        RETURNING owner_id INTO result.v_owner_id;
    END IF;

    -- Insert the repository using the obtained owner_id
    INSERT INTO repos (repo_full_name, repo_owner_id)
    VALUES (p_repo_full_name, result.v_owner_id)
    RETURNING repo_id INTO result.v_repo_id;

    RETURN result;
END;
-- +goose statementend

-- +goose down
DROP FUNCTION insert_repository;
DROP TYPE insert_repository_result;
