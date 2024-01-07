using CSharpServer.hubs;

var builder = WebApplication.CreateBuilder(args);

builder.Services.AddSignalR();
var app = builder.Build();

app.MapHub<Chat>("/chat");

app.Run();

