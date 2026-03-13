import { createApi, fetchBaseQuery } from "@reduxjs/toolkit/query/react";
import { getApiURL } from "./util";

export interface IrcServer {
  elevatedUsers?: string[];
  regularUsers?: string[];
}

export interface Book {
  name: string;
  path: string;
  downloadLink: string;
  time: string;
}

export interface LogEntry {
  time: string;
  level: "info" | "warn" | "error";
  message: string;
}

export const openbooksApi = createApi({
  baseQuery: fetchBaseQuery({
    baseUrl: getApiURL().href,
    credentials: "include",
    mode: "cors"
  }),
  tagTypes: ["books", "servers"],
  endpoints: (builder) => ({
    getServers: builder.query<string[], null>({
      query: () => `servers`,
      transformResponse: (ircServers: IrcServer) => {
        return ircServers.elevatedUsers ?? [];
      }
    }),
    getBooks: builder.query<Book[], null>({
      query: () => `library`,
      providesTags: ["books"]
    }),
    deleteBook: builder.mutation<null, string>({
      query: (book) => ({
        url: `library/${book}`,
        method: "DELETE"
      }),
      invalidatesTags: ["books"]
    }),
    getLogs: builder.query<LogEntry[], null>({
      query: () => `logs`
    }),
    getVersion: builder.query<string, null>({
      query: () => `version`
    })
  })
});

export const {
  useGetServersQuery,
  useGetBooksQuery,
  useDeleteBookMutation,
  useGetLogsQuery,
  useGetVersionQuery
} = openbooksApi;
