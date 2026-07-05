# Phase 3: Vector Store
#
# This module handles reading and writing vector embeddings to/from PostgreSQL
# using the pgvector extension.
#
# Responsibilities:
#   save_embedding(session, content, source_type, embedding, pipeline_run_id)
#       Persists a text snippet and its vector representation to the
#       code_embeddings table.
#
#   similarity_search(session, query_vector, top_k)
#       Queries the code_embeddings table using pgvector's cosine distance
#       operator (<->) and returns the top_k most similar rows.
#
# Implementation arrives in Phase 3.
