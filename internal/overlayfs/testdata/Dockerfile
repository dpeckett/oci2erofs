FROM tianon/toybox:0.8.11

RUN mkdir /foo /baz \
  && ln -s /bar /foo/bar \
  && ln -s /baz /foo/baz \
  && echo 'hello world' > /baz/a
RUN echo 'hello world' > /foo/a
RUN echo 'foo bar baz' > /foo/b
RUN echo 'qux quux' > /foo/c
RUN mkdir /bar \
  && echo 'foo bar baz' > /bar/b
RUN rm /foo/b
RUN echo 'quuz corge' > /foo/b
RUN rm /foo/c