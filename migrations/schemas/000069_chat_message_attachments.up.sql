ALTER TABLE chat_message
    ADD COLUMN IF NOT EXISTS attachments JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN chat_message.attachments IS 'Chat file attachments metadata [{id,name,type,size,url,kind,storageKey,text,extractedText}]';
