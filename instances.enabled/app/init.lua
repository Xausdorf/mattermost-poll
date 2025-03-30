-- Creating polls space --
box.schema.space.create('polls', { if_not_exists = true })

box.space.polls:format({
    { name = 'ID', type = 'string' },
    { name = 'Question', type = 'string' },
    { name = 'Options', type = 'array' },
    { name = 'IsActive', type = 'boolean' },
    { name = 'Author', type = 'string' }
})

box.space.polls:create_index('primary', { parts = { 'ID' }, if_not_exists = true })

-- Creating answers space --
box.schema.space.create('answers', { if_not_exists = true })

box.space.answers:format({
    { name = 'ID', type = 'string' },
    { name = 'UserID', type = 'string' },
    { name = 'PollID', type = 'string' },
    { name = 'Vote', type = 'unsigned' }
})

box.space.answers:create_index('primary', { parts = { 'ID' }, if_not_exists = true })
box.space.answers:create_index('user_poll', { 
    parts = { 'UserID', 'PollID' }, 
    unique = true, 
    if_not_exists = true 
})
